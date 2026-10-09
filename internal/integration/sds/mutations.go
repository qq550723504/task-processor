package sds

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"task-processor/internal/integration/httpimage"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/pod"
	"time"
)

// MutationClient has no retry, login bootstrap, delete or order methods.
// Exact OSS hosts are deployment-owned protocol configuration, never request DTOs.
type MutationClient struct {
	read     *ReadbackClient
	http     *http.Client
	ossHosts map[string]bool
}

func NewMutationClient(h *http.Client, credentials CredentialSource, ossHosts []string) (*MutationClient, error) {
	r, e := NewReadbackClient(h, credentials)
	if e != nil {
		return nil, e
	}
	if len(ossHosts) < 1 || len(ossHosts) > 4 {
		return nil, pod.ErrUnavailable
	}
	allowed := map[string]bool{}
	for _, host := range ossHosts {
		if !strings.HasSuffix(host, ".aliyuncs.com") || !strings.Contains(host, ".oss-") || strings.ContainsAny(host, "/:?#@\r\n") {
			return nil, pod.ErrUnavailable
		}
		allowed[host] = true
	}
	copy := *r.http
	copy.Timeout = 45 * time.Second
	return &MutationClient{r, &copy, allowed}, nil
}
func permitValid(p *submission.SendPermit) bool {
	return p != nil && p.AttemptID != "" && p.ClaimToken != "" && time.Now().Before(p.LeaseExpiresAt)
}
func (c *MutationClient) credentials(ctx context.Context, p pod.Plan) (Credentials, error) {
	v, e := c.read.credentials.Current(ctx)
	if e != nil || !matchesBinding(v, p.Binding) {
		return Credentials{}, pod.ErrUnavailable
	}
	return v, nil
}
func (c *MutationClient) Upload(ctx context.Context, p pod.Plan, raw []byte, permit *submission.SendPermit) (pod.ObjectReceipt, error) {
	if ctx == nil || !permitValid(permit) || len(raw) < 1 || len(raw) > pod.MaxArtworkBytes {
		return pod.ObjectReceipt{}, pod.ErrInvalid
	}
	sum := sha256.Sum256(raw)
	if int64(len(raw)) != p.Artwork.Bytes || hex.EncodeToString(sum[:]) != p.Artwork.Hash {
		return pod.ObjectReceipt{}, pod.ErrConflict
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	credentials, e := c.credentials(ctx, p)
	if e != nil {
		return pod.ObjectReceipt{}, e
	}
	var sign struct {
		Dir       string `json:"dir"`
		Policy    string `json:"policy"`
		Key       string `json:"ossAccessKeyId"`
		Signature string `json:"signature"`
		Host      string `json:"host"`
	}
	if c.read.get(ctx, credentials, "mapi.sdspod.com", "/ps/image/get_post_signature_to_image_for_oss_upload", nil, &sign) != nil {
		return pod.ObjectReceipt{}, pod.ErrUnknown
	}
	u, e := url.Parse(sign.Host)
	if e != nil || u.Scheme != "https" || !c.ossHosts[u.Host] || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" || len(sign.Dir) > 256 || sign.Dir == "" || strings.ContainsAny(sign.Dir, "\\\x00") || strings.Contains(sign.Dir, "..") || sign.Policy == "" || len(sign.Policy) > 8192 || sign.Key == "" || sign.Signature == "" {
		return pod.ObjectReceipt{}, pod.ErrUnknown
	}
	ext := ".png"
	if p.Artwork.MediaType == "image/jpeg" {
		ext = ".jpg"
	}
	md := md5.Sum(raw)
	file := hex.EncodeToString(md[:]) + ext
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{"key": sign.Dir + file, "policy": sign.Policy, "OSSAccessKeyId": sign.Key, "success_action_status": "200", "signature": sign.Signature} {
		if writer.WriteField(key, value) != nil {
			return pod.ObjectReceipt{}, pod.ErrUnknown
		}
	}
	part, e := writer.CreateFormFile("file", file)
	if e != nil {
		return pod.ObjectReceipt{}, pod.ErrUnknown
	}
	if _, e = part.Write(raw); e != nil || writer.Close() != nil {
		return pod.ObjectReceipt{}, pod.ErrUnknown
	}
	if _, e = c.credentials(ctx, p); e != nil || !permitValid(permit) {
		return pod.ObjectReceipt{}, pod.ErrUnknown
	}
	request, e := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body.Bytes()))
	if e != nil {
		return pod.ObjectReceipt{}, pod.ErrUnknown
	}
	request.GetBody = nil
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, e := c.http.Do(request)
	if e != nil {
		return pod.ObjectReceipt{}, pod.ErrUnknown
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return pod.ObjectReceipt{}, pod.ErrUnknown
	}
	if _, e = io.Copy(io.Discard, io.LimitReader(response.Body, 65537)); e != nil {
		return pod.ObjectReceipt{}, pod.ErrUnknown
	}
	return pod.ObjectReceipt{FileCode: file, Hash: p.Artwork.Hash}, nil
}

type materialDTO struct {
	ID        remoteID `json:"id"`
	Name      string   `json:"name"`
	FileCode  string   `json:"file_code"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	MediaType string   `json:"content_type"`
	URL       string   `json:"img_url"`
	AltURL    string   `json:"imgUrl"`
}

func (c *MutationClient) CreateMaterial(ctx context.Context, p pod.Plan, object pod.ObjectReceipt, permit *submission.SendPermit) (pod.MaterialReceipt, error) {
	if ctx == nil || !permitValid(permit) || object.Hash != p.Artwork.Hash || object.FileCode == "" || strings.ContainsAny(object.FileCode, "/\\?#\x00") {
		return pod.MaterialReceipt{}, pod.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	payload, _ := json.Marshal(map[string]any{"file_code": object.FileCode, "length": p.Artwork.Bytes, "name": pod.MaterialName(p.OperationID), "content_type": p.Artwork.MediaType, "width": p.Artwork.Width, "height": p.Artwork.Height, "parent_folder_id": 0, "repeatReturnId": true})
	raw, e := c.post(ctx, p, "/materials/one", payload, permit)
	if e != nil {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	var result struct {
		Ret  *int          `json:"ret"`
		Data []materialDTO `json:"data"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Ret == nil || *result.Ret != 0 || len(result.Data) != 1 {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	m := result.Data[0]
	if !validRemoteID(string(m.ID)) || m.Name != pod.MaterialName(p.OperationID) || m.FileCode != object.FileCode || m.Width != p.Artwork.Width || m.Height != p.Artwork.Height || m.MediaType != p.Artwork.MediaType {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	credentials, e := c.credentials(ctx, p)
	if e != nil {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	var confirmed []materialDTO
	if c.read.get(ctx, credentials, "mapi.sdspod.com", "/materials/findByIds", url.Values{"ids": {string(m.ID)}, "fields": {"id,name,imgUrl,width,height,file_code,content_type"}}, &confirmed) != nil || len(confirmed) != 1 {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	v := confirmed[0]
	if v.ID != m.ID || v.Name != m.Name || v.FileCode != m.FileCode || v.Width != m.Width || v.Height != m.Height || v.MediaType != m.MediaType {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	image := v.AltURL
	if image == "" {
		image = v.URL
	}
	u, e := url.Parse(image)
	if e != nil || u.Scheme != "https" || u.Host != "cdn.sdspod.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(u.Path, "/"+object.FileCode) {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	if strings.HasPrefix(u.Path, "/images/") {
		u.Path = "/images1000Thumbs/" + strings.TrimPrefix(u.Path, "/images/")
	}
	if !strings.HasPrefix(u.Path, "/images1000Thumbs/") {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	q := url.Values{"material_id": {string(m.ID)}}
	u.RawQuery = q.Encode()
	// Read the actual provider thumbnail dimensions. Original artwork dimensions
	// cannot be used for its downscaled Fabric image object.
	r, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	response, e := c.read.http.Do(r)
	if e != nil {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	defer response.Body.Close()
	thumb, e := io.ReadAll(io.LimitReader(response.Body, pod.MaxArtworkBytes+1))
	if e != nil || response.StatusCode != 200 || len(thumb) > pod.MaxArtworkBytes {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	_, w, h, e := httpimage.InspectGeneratedArtifact(thumb)
	if e != nil || w < 1 || h < 1 || w > 10000 || h > 10000 || int64(w)*int64(h) > 40000000 {
		return pod.MaterialReceipt{}, pod.ErrUnknown
	}
	return pod.MaterialReceipt{ID: string(m.ID), Name: m.Name, FileCode: m.FileCode, URL: u.String(), Hash: p.Artwork.Hash, Width: w, Height: h}, nil
}
func (c *MutationClient) Sync(ctx context.Context, p pod.Plan, payload []byte, permit *submission.SendPermit) error {
	if ctx == nil || !permitValid(permit) || len(payload) < 1 || len(payload) > 2<<20 || !json.Valid(payload) {
		return pod.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	_, _ = c.post(ctx, p, "/ps/design/syncDesign", payload, permit)
	// The qualified endpoint returns HTTP200 with no identity or receipt. Every
	// response stays unknown until exact saved-design/task/render observation.
	return pod.ErrUnknown
}
func (c *MutationClient) post(ctx context.Context, p pod.Plan, path string, payload []byte, permit *submission.SendPermit) ([]byte, error) {
	if path != "/materials/one" && path != "/ps/design/syncDesign" || !permitValid(permit) {
		return nil, pod.ErrInvalid
	}
	credentials, e := c.credentials(ctx, p)
	if e != nil {
		return nil, e
	}
	u := url.URL{Scheme: "https", Host: "mapi.sdspod.com", Path: path}
	if path == "/materials/one" {
		u.RawQuery = "t=" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	}
	r, e := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if e != nil {
		return nil, pod.ErrUnknown
	}
	r.GetBody = nil
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Access-Token", credentials.AccessToken)
	r.Header.Set("Origin", "https://www.sdsdiy.com")
	r.Header.Set("Referer", "https://www.sdsdiy.com/")
	if credentials.OutToken != "" {
		r.Header.Set("Out-Access-Token", credentials.OutToken)
	}
	response, e := c.http.Do(r)
	if e != nil {
		return nil, pod.ErrUnknown
	}
	defer response.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(response.Body, 2<<20+1))
	if e != nil || len(raw) > 2<<20 || response.StatusCode != 200 {
		return nil, pod.ErrUnknown
	}
	return raw, nil
}
