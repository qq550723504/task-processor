package sds

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"task-processor/internal/product/pod"
	"time"
)

type TemplateClient struct{ read *ReadbackClient }

func NewTemplateClient(http *http.Client, credentials CredentialSource) (*TemplateClient, error) {
	r, e := NewReadbackClient(http, credentials)
	if e != nil {
		return nil, e
	}
	return &TemplateClient{r}, nil
}
func (c *TemplateClient) List(ctx context.Context, page, size int, keyword string) (pod.TemplatePage, error) {
	if ctx == nil || page < 1 || page > 10000 || size < 1 || size > 50 || len(keyword) > 200 || strings.ContainsAny(keyword, "\x00\r\n") {
		return pod.TemplatePage{}, pod.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var dto struct {
		Total int64         `json:"totalCount"`
		Page  int           `json:"page"`
		Size  int           `json:"size"`
		Items []templateDTO `json:"items"`
	}
	if c.read.get(ctx, Credentials{}, "mapi.sdspod.com", "/products/page", url.Values{"page": {strconv.Itoa(page)}, "size": {strconv.Itoa(size)}, "keyword": {keyword}}, &dto) != nil || dto.Total < 0 || len(dto.Items) > size {
		return pod.TemplatePage{}, pod.ErrUnknown
	}
	result := pod.TemplatePage{Items: []pod.Template{}, Total: dto.Total, Page: page, Size: size}
	for _, item := range dto.Items {
		t, e := item.template()
		if e != nil {
			return pod.TemplatePage{}, e
		}
		result.Items = append(result.Items, t)
	}
	return result, nil
}
func (c *TemplateClient) Detail(ctx context.Context, id string) (pod.Template, error) {
	if ctx == nil || !validRemoteID(id) {
		return pod.Template{}, pod.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var dto templateDTO
	if c.read.get(ctx, Credentials{}, "mapi.sdspod.com", "/products/"+id, nil, &dto) != nil || string(dto.ID) != id {
		return pod.Template{}, pod.ErrUnknown
	}
	return dto.template()
}
func (c *TemplateClient) Manifest(ctx context.Context, b pod.AccountBinding, parent, variant string) (pod.TemplateManifest, error) {
	if ctx == nil || !validRemoteID(parent) || !validRemoteID(variant) || b.ProtocolRevision != pod.ProtocolRevision {
		return pod.TemplateManifest{}, pod.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	creds, e := c.read.credentials.Current(ctx)
	if e != nil || !matchesBinding(creds, b) {
		return pod.TemplateManifest{}, pod.ErrUnavailable
	}
	var d struct {
		ActualProduct struct {
			ID        remoteID `json:"id"`
			Parent    remoteID `json:"parent_id"`
			Prototype remoteID `json:"prototypeId"`
			Type      string   `json:"prototypeType"`
		} `json:"product"`
		Group struct {
			ID     remoteID `json:"id"`
			Parent remoteID `json:"productId"`
		} `json:"prototypeGroup"`
		Layers []struct {
			ID               remoteID `json:"id"`
			Prototype        remoteID `json:"prototypeId"`
			Type             int      `json:"type"`
			Width, Height    int
			PrintWidth       int `json:"print_width"`
			PrintHeight      int `json:"print_height"`
			PrintWidthCamel  int `json:"printWidth"`
			PrintHeightCamel int `json:"printHeight"`
		} `json:"layers"`
		Files []struct {
			ID        remoteID `json:"id"`
			Prototype remoteID `json:"prototypeId"`
			Thumbnail string   `json:"thumbnail_url"`
		} `json:"psds"`
	}
	if c.read.get(ctx, creds, "mapi.sdspod.com", "/ps/design/products/"+variant, nil, &d) != nil || string(d.ActualProduct.ID) != variant || string(d.ActualProduct.Parent) != parent {
		return pod.TemplateManifest{}, pod.ErrUnknown
	}
	m := pod.TemplateManifest{ParentID: parent, VariantID: variant, PrototypeID: string(d.ActualProduct.Prototype), GroupID: string(d.Group.ID), Type: d.ActualProduct.Type}
	if m.Type != "FREE" || string(d.Group.Parent) != parent {
		return m, pod.ErrUnavailable
	}
	for _, l := range d.Layers {
		if l.Type != 1 || l.Prototype != d.ActualProduct.Prototype {
			return m, pod.ErrUnavailable
		}
		pw, ph := l.PrintWidth, l.PrintHeight
		if pw == 0 {
			pw = l.PrintWidthCamel
		}
		if ph == 0 {
			ph = l.PrintHeightCamel
		}
		if l.PrintWidthCamel != 0 && l.PrintWidthCamel != pw || l.PrintHeightCamel != 0 && l.PrintHeightCamel != ph {
			return m, pod.ErrUnknown
		}
		m.Layers = append(m.Layers, pod.EditableLayer{ID: string(l.ID), Width: l.Width, Height: l.Height, PrintWidth: pw, PrintHeight: ph})
	}
	for _, f := range d.Files {
		if f.Prototype != d.ActualProduct.Prototype {
			return m, pod.ErrUnknown
		}
		m.RenderFiles = append(m.RenderFiles, pod.RenderFile{ID: string(f.ID), Thumbnail: f.Thumbnail})
	}
	if e = m.Validate(); e != nil {
		return m, e
	}
	return m, nil
}
func matchesBinding(c Credentials, b pod.AccountBinding) bool {
	return validCredentials(c) && c.BindingID == b.ID && c.Revision == b.Revision && c.MerchantID == b.MerchantID && b.ProtocolRevision == pod.ProtocolRevision
}

type templateDTO struct {
	ID          remoteID `json:"id"`
	Name        string   `json:"name"`
	ProductName string   `json:"product_name"`
	SKU         string   `json:"sku"`
	Type        string   `json:"prototypeType"`
	Size        string   `json:"size"`
	Color       string   `json:"color_name"`
	Image       string   `json:"img_url"`
	Thumb       string   `json:"thumbImgUrl"`
	Blank       string   `json:"blankDesignUrl"`
	Subproducts *struct {
		Items []templateDTO `json:"items"`
	} `json:"subproducts"`
}

func (d templateDTO) template() (pod.Template, error) {
	name := d.Name
	if name == "" {
		name = d.ProductName
	}
	t := pod.Template{ID: string(d.ID), Name: name, SKU: d.SKU, Images: []string{}, Variants: []pod.TemplateVariant{}}
	for _, im := range []string{d.Image, d.Thumb, d.Blank} {
		if im != "" {
			u, e := url.Parse(im)
			if e != nil || u.Scheme != "https" || u.Host != "cdn.sdspod.com" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
				continue
			}
			seen := false
			for _, old := range t.Images {
				seen = seen || old == im
			}
			if !seen {
				t.Images = append(t.Images, im)
			}
		}
	}
	if d.Subproducts != nil {
		if len(d.Subproducts.Items) > 256 {
			return t, pod.ErrUnknown
		}
		for _, v := range d.Subproducts.Items {
			vn := v.Name
			if vn == "" {
				vn = v.ProductName
			}
			images := []string{}
			u, e := url.Parse(v.Image)
			if e == nil && u.Scheme == "https" && u.Host == "cdn.sdspod.com" && u.RawQuery == "" && u.User == nil && u.Fragment == "" {
				images = append(images, v.Image)
			}
			t.Variants = append(t.Variants, pod.TemplateVariant{ID: string(v.ID), Name: vn, SKU: v.SKU, Type: v.Type, Size: v.Size, Color: v.Color, Images: images})
		}
	}
	return t, t.Validate()
}
