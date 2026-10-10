// Package sds implements the qualified current SDS protocol without legacy
// workflow, automatic login, write retry, latest-product lookup or cleanup.
package sds

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/product/pod"
)

const maxReadbackBytes = 2 << 20

type Binding struct{ ID, Revision, MerchantID string }

// Credentials are injected by a server-only private configuration owner.
// Current must return the attested merchant identity for this credential revision.
// Browser sessions and request bodies are never credential sources.
type Credentials struct {
	BindingID, Revision, MerchantID string
	AccessToken                     string `json:"-"`
	OutToken                        string `json:"-"`
}

func (Credentials) String() string   { return "SDS credentials (redacted)" }
func (Credentials) GoString() string { return "SDS credentials (redacted)" }

type CredentialSource interface {
	Current(context.Context) (Credentials, error)
}
type ReadbackClient struct {
	http        *http.Client
	credentials CredentialSource
}

func NewReadbackClient(client *http.Client, credentials CredentialSource) (*ReadbackClient, error) {
	if client == nil || credentials == nil {
		return nil, pod.ErrUnavailable
	}
	copy := *client
	copy.Jar = nil
	if copy.Timeout <= 0 || copy.Timeout > 15*time.Second {
		copy.Timeout = 15 * time.Second
	}
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &ReadbackClient{&copy, credentials}, nil
}

// Observe performs only bounded GETs. An empty successful sync response cannot
// become a receipt: the caller must supply the original frozen design intent.
func (c *ReadbackClient) Observe(ctx context.Context, b Binding, i pod.DesignIntent) (pod.QualifiedFinished, error) {
	if ctx == nil || c == nil || c.credentials == nil || i.MerchantID != b.MerchantID {
		return pod.QualifiedFinished{}, pod.ErrUnknown
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	creds, err := c.credentials.Current(ctx)
	if err != nil || creds.BindingID != b.ID || creds.Revision != b.Revision || creds.MerchantID != b.MerchantID || !validCredentials(creds) || !authidentity.IsBoundedIdentifier(i.OperationID) {
		return pod.QualifiedFinished{}, pod.ErrUnknown
	}
	query := url.Values{"1": {"1"}, "designType": {"material"}, "size": {"10"}, "page": {"1"}, "search": {i.OperationID}, "lifecycleType": {"live"}, "category_id": {"0"}, "isMicroCustomize": {"no"}}
	var page struct {
		Items []finishedDTO `json:"items"`
		Total int           `json:"total_count"`
	}
	if c.get(ctx, creds, "mapi2.sdspod.com", "/design_products", query, &page) != nil || page.Total != 1 || len(page.Items) != 1 {
		return pod.QualifiedFinished{}, pod.ErrUnknown
	}
	listed := page.Items[0]
	if !listed.BuildFinish || !validRemoteID(string(listed.ID)) {
		return pod.QualifiedFinished{}, pod.ErrUnknown
	}
	var detail finishedDTO
	if c.get(ctx, creds, "mapi.sdspod.com", "/design_products/"+string(listed.ID), nil, &detail) != nil || !reflect.DeepEqual(listed.record(), detail.record()) {
		return pod.QualifiedFinished{}, pod.ErrUnknown
	}
	var design savedDesignDTO
	if c.get(ctx, creds, "mapi.sdspod.com", "/ps/endproducts/"+string(listed.ID)+"/design", nil, &design) != nil {
		return pod.QualifiedFinished{}, pod.ErrUnknown
	}
	var task *taskDTO
	for pageNum := 1; pageNum <= 4 && task == nil; pageNum++ {
		var tasks struct {
			Total int       `json:"total"`
			Items []taskDTO `json:"items"`
		}
		if c.get(ctx, creds, "mapi.sdspod.com", "/ps/task/result", url.Values{"page": {strconv.Itoa(pageNum)}, "limit": {"50"}}, &tasks) != nil || len(tasks.Items) > 50 {
			return pod.QualifiedFinished{}, pod.ErrUnknown
		}
		for index := range tasks.Items {
			if tasks.Items[index].ID == listed.TaskID {
				if task != nil {
					return pod.QualifiedFinished{}, pod.ErrUnknown
				}
				copy := tasks.Items[index]
				task = &copy
			}
		}
		if len(tasks.Items) < 50 || pageNum*50 >= tasks.Total {
			break
		}
	}
	if task == nil || !validRemoteID(string(task.ID)) {
		return pod.QualifiedFinished{}, pod.ErrUnknown
	}
	var children struct {
		TaskID remoteID `json:"taskId"`
		Items  []struct {
			ProductID remoteID `json:"productId"`
			ParentID  remoteID `json:"productParentId"`
			Results   []struct {
				File string `json:"file"`
			} `json:"results"`
		} `json:"items"`
	}
	if c.get(ctx, creds, "mapi.sdspod.com", "/ps/task/"+string(task.ID)+"/children", nil, &children) != nil || children.TaskID != task.ID || len(children.Items) != 1 || string(children.Items[0].ProductID) != i.VariantID || string(children.Items[0].ParentID) != i.ParentID {
		return pod.QualifiedFinished{}, pod.ErrUnknown
	}
	f := listed.record()
	f.BuildFinish = true
	o := pod.Observation{Finished: f, Design: design.record(), Task: pod.TaskRecord{ID: string(task.ID), Status: task.Status, Complete: task.Complete, Success: task.Success, Failed: task.Failed, Wait: task.Wait}}
	for _, result := range children.Items[0].Results {
		o.Task.RenderURLs = append(o.Task.RenderURLs, result.File)
	}
	return pod.QualifyFinished(i, []pod.Observation{o})
}

func validCredentials(c Credentials) bool {
	return authidentity.IsBoundedIdentifier(c.BindingID) && authidentity.IsBoundedIdentifier(c.Revision) && validRemoteID(c.MerchantID) && len(c.AccessToken) > 0 && len(c.AccessToken) <= 8192 && len(c.OutToken) <= 8192 && !strings.ContainsAny(c.AccessToken+c.OutToken, "\r\n\x00")
}
func (c *ReadbackClient) get(ctx context.Context, credentials Credentials, host, path string, query url.Values, out any) error {
	if (host != "mapi.sdspod.com" && host != "mapi2.sdspod.com") || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#\r\n") {
		return pod.ErrUnknown
	}
	u := url.URL{Scheme: "https", Host: host, Path: path, RawQuery: query.Encode()}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return pod.ErrUnknown
	}
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Origin", "https://www.sdsdiy.com")
	r.Header.Set("Referer", "https://www.sdsdiy.com/")
	r.Header.Set("Access-Token", credentials.AccessToken)
	if credentials.OutToken != "" {
		r.Header.Set("Out-Access-Token", credentials.OutToken)
	}
	response, err := c.http.Do(r)
	if err != nil {
		return pod.ErrUnknown
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return pod.ErrUnknown
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxReadbackBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > maxReadbackBytes || json.Unmarshal(raw, out) != nil {
		return pod.ErrUnknown
	}
	return nil
}

// SDS currently mixes JSON numbers and decimal strings for provider IDs. No
// floating conversion is permitted, because its snowflake IDs exceed 2^53.
type remoteID string

func (id *remoteID) UnmarshalJSON(raw []byte) error {
	var value string
	if len(raw) > 0 && raw[0] == '"' {
		if json.Unmarshal(raw, &value) != nil {
			return pod.ErrUnknown
		}
	} else {
		value = string(raw)
	}
	if value != "0" && !validRemoteID(value) {
		return pod.ErrUnknown
	}
	*id = remoteID(value)
	return nil
}
func validRemoteID(value string) bool {
	if len(value) == 0 || len(value) > 64 || value[0] == '0' {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

type finishedDTO struct {
	ID           remoteID `json:"id"`
	KeyID        string   `json:"key_id"`
	MerchantID   remoteID `json:"merchant_id"`
	TaskID       remoteID `json:"task_id"`
	DesignTaskID remoteID `json:"design_task_id"`
	ParentID     remoteID `json:"product_parent_id"`
	VariantID    remoteID `json:"product_id"`
	PrototypeID  remoteID `json:"prototype_id"`
	BuildFinish  bool     `json:"buildFinish"`
	Status       int      `json:"status"`
	RenderURLs   []string `json:"img_urls"`
}

func (f finishedDTO) record() pod.FinishedRecord {
	return pod.FinishedRecord{ID: string(f.ID), KeyID: f.KeyID, MerchantID: string(f.MerchantID), TaskID: string(f.TaskID), DesignTaskID: string(f.DesignTaskID), ParentID: string(f.ParentID), VariantID: string(f.VariantID), PrototypeID: string(f.PrototypeID), Status: f.Status, RenderURLs: f.RenderURLs}
}

type savedDesignDTO struct {
	FinishedID remoteID `json:"designProductId"`
	Product    struct {
		ID       remoteID `json:"id"`
		ParentID remoteID `json:"parent_id"`
	} `json:"product"`
	Group struct {
		ID remoteID `json:"id"`
	} `json:"prototypeGroup"`
	PrototypeID remoteID `json:"prototype_id"`
	Layers      []struct {
		ID          remoteID `json:"id"`
		Width       int      `json:"width"`
		Height      int      `json:"height"`
		PrintWidth  int      `json:"print_width"`
		PrintHeight int      `json:"print_height"`
		FabricJSON  string   `json:"fabric_json"`
	} `json:"layers"`
	Files []struct {
		ID remoteID `json:"id"`
	} `json:"designFiles"`
}

func (d savedDesignDTO) record() pod.SavedDesign {
	r := pod.SavedDesign{FinishedID: string(d.FinishedID), ParentID: string(d.Product.ParentID), VariantID: string(d.Product.ID), PrototypeID: string(d.PrototypeID), GroupID: string(d.Group.ID)}
	for _, l := range d.Layers {
		r.Layers = append(r.Layers, pod.DesignLayer{ID: string(l.ID), Width: l.Width, Height: l.Height, PrintWidth: l.PrintWidth, PrintHeight: l.PrintHeight, FabricJSON: l.FabricJSON})
	}
	for _, f := range d.Files {
		r.RenderFileIDs = append(r.RenderFileIDs, string(f.ID))
	}
	return r
}

type taskDTO struct {
	ID       remoteID `json:"id"`
	Status   int      `json:"status"`
	Complete int      `json:"qty_complete"`
	Success  int      `json:"qty_success"`
	Failed   int      `json:"qty_failed"`
	Wait     int      `json:"qty_wait"`
}
