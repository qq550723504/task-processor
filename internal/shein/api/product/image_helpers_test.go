package product

import "testing"

func TestCollectSizeMapImageURLs(t *testing.T) {
	product := &Product{
		ImageInfo: &ImageInfo{
			ImageInfoList: []ImageDetail{
				{ImageURL: "https://cdn.example.com/main.jpg"},
				{ImageURL: "https://cdn.example.com/size.jpg", SizeImgFlag: true},
			},
		},
		SKCList: []SKC{{
			ImageInfo: ImageInfo{
				ImageInfoList: []ImageDetail{
					{ImageURL: "https://cdn.example.com/skc-size.jpg", SizeImgFlag: true},
				},
			},
		}},
	}

	urls := CollectSizeMapImageURLs(product)
	if len(urls) != 2 {
		t.Fatalf("size map url count = %d, want 2", len(urls))
	}
	if _, ok := urls["https://cdn.example.com/size.jpg"]; !ok {
		t.Fatalf("missing spu size map url: %+v", urls)
	}
	if _, ok := urls["https://cdn.example.com/skc-size.jpg"]; !ok {
		t.Fatalf("missing skc size map url: %+v", urls)
	}
}
