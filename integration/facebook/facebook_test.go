package facebook

import (
	"os"
	"strings"
	"testing"
)

func TestReadJSONString(t *testing.T) {
	got, err := readJSONString(`"https:\/\/video.fbcdn.net\/foo.mp4?x=1"`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://video.fbcdn.net/foo.mp4?x=1" {
		t.Fatalf("got %q", got)
	}
}

func TestParseOGTitle(t *testing.T) {
	author, caption := parseOGTitle("18M views · 188K reactions | Hand feeding bass | Kevin 4K")
	if author != "Kevin 4K" {
		t.Fatalf("author=%q", author)
	}
	if caption != "Hand feeding bass" {
		t.Fatalf("caption=%q", caption)
	}
}

func TestIsProfileURL(t *testing.T) {
	tests := map[string]bool{
		"https://www.facebook.com/NASA":                             true,
		"https://www.facebook.com/people/Kevin-4K/100090658043727/": true,
		"https://www.facebook.com/taylormakesvideos":                true,
		"https://www.facebook.com/NASA/posts/pfbid02abc":            false,
		"https://www.facebook.com/reel/730293269054758/":            false,
		"https://www.facebook.com/watch/?v=123":                     false,
		"https://www.facebook.com/hashtag/reels":                    false,
		"https://www.facebook.com/marketplace/item/123":             false,
		"https://www.facebook.com/marketplace":                      false,
	}
	for raw, want := range tests {
		if got := isProfileURL(raw); got != want {
			t.Errorf("isProfileURL(%q)=%v want %v", raw, got, want)
		}
	}
}

func TestCleanPostURL(t *testing.T) {
	got := cleanPostURL("https://www.facebook.com/watch/?v=883839773514682&ref=sharing&rdid=abc")
	if got != "https://www.facebook.com/watch/?v=883839773514682" {
		t.Fatalf("got %q", got)
	}

	got = cleanPostURL("https://www.facebook.com/NASA/photos/foo/1613324343496269/?ref=share")
	if got != "https://www.facebook.com/NASA/photos/foo/1613324343496269/" {
		t.Fatalf("got %q", got)
	}
}

func TestParseHTMLVideo(t *testing.T) {
	htmlBody := `
<title>Facebook</title>
<meta property="og:title" content="1.1M views · 10K reactions | Sebaskom full | Wood" />
<meta property="og:description" content="Sebaskom full" />
<meta property="og:url" content="https://www.facebook.com/wood57/videos/883839773514682/" />
<link rel="canonical" href="https://www.facebook.com/wood57/videos/883839773514682/" />
{"actors":[{"__typename":"User","name":"Wood","id":"1","__isActor":"User","url":"https:\/\/www.facebook.com\/wood57"}]}
"message":{"text":"Sebaskom full","ranges":[]}
"browser_native_hd_url":"https:\/\/video.fbcdn.net\/o1\/v\/hd.mp4?oh=1"
"browser_native_sd_url":"https:\/\/video.fbcdn.net\/o1\/v\/sd.mp4?oh=1"
`
	got := parseHTML(htmlBody, "https://www.facebook.com/watch/?v=883839773514682")
	if got.AuthorName != "Wood" {
		t.Fatalf("author name=%q", got.AuthorName)
	}
	if got.AuthorURL != "https://www.facebook.com/wood57" {
		t.Fatalf("author url=%q", got.AuthorURL)
	}
	if got.Caption != "Sebaskom full" {
		t.Fatalf("caption=%q", got.Caption)
	}
	if len(got.MediaDetails) != 1 || got.MediaDetails[0].Type != "video" {
		t.Fatalf("media=%+v", got.MediaDetails)
	}
	if got.MediaDetails[0].URL != "https://video.fbcdn.net/o1/v/hd.mp4?oh=1" {
		t.Fatalf("video url=%q", got.MediaDetails[0].URL)
	}
	if got.PostURL != "https://www.facebook.com/wood57/videos/883839773514682/" {
		t.Fatalf("post url=%q", got.PostURL)
	}
}

func TestParseHTMLPhoto(t *testing.T) {
	htmlBody := `
<link rel="canonical" href="https://www.facebook.com/NASA/photos/hello/1613324343496269/" />
<meta name="description" content="NASA honors Dolly" />
{"__isActor":"User","name":"NASA - National Aeronautics and Space Administration","url":"https:\/\/www.facebook.com\/NASA"}
"message":{"text":"NASA honors the memory of Dolly Parton"}
"image":{"uri":"https:\/\/scontent.fbcdn.net\/v\/t39\/photo_n.png?stp=dst-jpg&oh=abc"}
`
	got := parseHTML(htmlBody, "https://www.facebook.com/photo/?fbid=1613324343496269")
	if got.AuthorName != "NASA - National Aeronautics and Space Administration" {
		t.Fatalf("author name=%q", got.AuthorName)
	}
	if got.AuthorURL != "https://www.facebook.com/NASA" {
		t.Fatalf("author url=%q", got.AuthorURL)
	}
	if got.Caption != "NASA honors the memory of Dolly Parton" {
		t.Fatalf("caption=%q", got.Caption)
	}
	if len(got.MediaDetails) != 1 || got.MediaDetails[0].Type != "photo" {
		t.Fatalf("media=%+v", got.MediaDetails)
	}
	if got.PostURL != "https://www.facebook.com/NASA/photos/hello/1613324343496269/" {
		t.Fatalf("post url=%q", got.PostURL)
	}
}

func TestPickVideoPrefersCDNOverLookaside(t *testing.T) {
	htmlBody := `
"browser_native_hd_url":"https:\/\/lookaside.fbsbx.com\/lookaside\/crawler\/media\/?media_id=1"
"browser_native_sd_url":"https:\/\/video.fbcdn.net\/o1\/v\/sd.mp4?oh=1"
`
	got := pickVideoURL(htmlBody)
	if got != "https://video.fbcdn.net/o1/v/sd.mp4?oh=1" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractPhotoSkipsTinyAndCaps(t *testing.T) {
	htmlBody := `
"image":{"uri":"https:\/\/scontent.fbcdn.net\/v\/t39\/avatar.jpg?ctp=s32x32"}
"image":{"uri":"https:\/\/scontent.fbcdn.net\/v\/t39\/one.jpg?oh=1"}
"image":{"uri":"https:\/\/scontent.fbcdn.net\/v\/t39\/one.jpg?oh=2"}
`
	got := extractPhotoURLs(htmlBody, "")
	if len(got) != 1 {
		t.Fatalf("got %d photos: %v", len(got), got)
	}
}

func TestParseHTMLMarketplace(t *testing.T) {
	htmlBody := `
<link rel="canonical" href="https://www.facebook.com/marketplace/item/1201486206389924/" />
<meta property="og:title" content="Top preto" />
<meta property="og:description" content="Top preto&#10;&#10;TAM. Único" />
<meta property="og:url" content="https://www.facebook.com/marketplace/item/1201486206389924/" />
<meta property="og:image" content="https://scontent.fbcdn.net/v/t39/related.jpg" />
"marketplace_listing_title":"Bicicleta ergometrica"
"listing_photos":[{"__typename":"ProductImage","image":{"height":960,"width":818,"uri":"https:\/\/scontent.fbcdn.net\/v\/t39.84726-6\/item1.jpg?oh=1"},"id":"1"},{"__typename":"ProductImage","image":{"uri":"https:\/\/scontent.fbcdn.net\/v\/t45.5328-4\/item2.jpg?oh=2"},"id":"2"}]
"redacted_description":{"text":"Top preto\n\nTAM. Único\n\nApenas $ 10,00"},"creation_time":1,"location_text":{"text":"Rio de Janeiro, RJ"},"listing_price":{"formatted_amount_zeros_stripped":"R$10","amount":"10.00","currency":"BRL"},"base_marketplace_listing_title":"Top preto","marketplace_listing_title":"Top preto"
`
	got := parseHTML(htmlBody, "https://www.facebook.com/marketplace/item/1201486206389924/?ref=browse_tab")
	if !got.IsMarketplace {
		t.Fatal("expected marketplace listing")
	}
	if got.Title != "Top preto" {
		t.Fatalf("title=%q", got.Title)
	}
	if got.Price != "R$10" {
		t.Fatalf("price=%q", got.Price)
	}
	if got.Location != "Rio de Janeiro, RJ" {
		t.Fatalf("location=%q", got.Location)
	}
	if !strings.Contains(got.Caption, "TAM. Único") {
		t.Fatalf("caption=%q", got.Caption)
	}
	if got.PostURL != "https://www.facebook.com/marketplace/item/1201486206389924/" {
		t.Fatalf("post url=%q", got.PostURL)
	}
	if len(got.MediaDetails) != 2 {
		t.Fatalf("media=%+v", got.MediaDetails)
	}
	if !strings.Contains(got.MediaDetails[0].URL, "item1.jpg") {
		t.Fatalf("first photo=%q", got.MediaDetails[0].URL)
	}
	if !strings.Contains(got.MediaDetails[1].URL, "item2.jpg") {
		t.Fatalf("second photo=%q", got.MediaDetails[1].URL)
	}
}

func TestCanonicalizeURL(t *testing.T) {
	got := CanonicalizeURL("https://www.facebook.com/marketplace/item/1201486206389924/?ref=browse_tab&referral_code=marketplace_top_picks&referral_story_type=top_picks")
	if got != "https://www.facebook.com/marketplace/item/1201486206389924/" {
		t.Fatalf("got %q", got)
	}

	got = CanonicalizeURL("https://l.facebook.com/l.php?u=https%3A%2F%2Fwww.facebook.com%2Fmarketplace%2Fitem%2F1201486206389924%2F&h=AT")
	if got != "https://www.facebook.com/marketplace/item/1201486206389924/" {
		t.Fatalf("unwrap got %q", got)
	}
}

func TestParseHTMLMarketplaceOGOnly(t *testing.T) {
	htmlBody := `
<title>Top preto — Tops &amp; T-Shirts — Rio de Janeiro | Facebook Marketplace</title>
<meta property="og:title" content="Top preto" />
<meta property="og:description" content="Top preto

TAM. Único

Apenas $ 10,00" />
<meta property="og:url" content="https://www.facebook.com/marketplace/item/1201486206389924/" />
<meta property="og:image" content="https://scontent.fbcdn.net/v/t39/item.jpg?oh=1" />
`
	got := parseHTML(htmlBody, "https://www.facebook.com/marketplace/item/1201486206389924/")
	if !got.IsMarketplace {
		t.Fatal("expected marketplace listing from og tags")
	}
	if got.Title != "Top preto" {
		t.Fatalf("title=%q", got.Title)
	}
	if got.Price != "$ 10,00" {
		t.Fatalf("price=%q", got.Price)
	}
	if got.Location != "Rio de Janeiro" {
		t.Fatalf("location=%q", got.Location)
	}
	if len(got.MediaDetails) != 1 {
		t.Fatalf("media=%+v", got.MediaDetails)
	}
}

func TestIsLoginWall(t *testing.T) {
	if !isLoginWall(`<title>Log into Facebook</title><meta property="og:title" content="Facebook" />`) {
		t.Fatal("expected login wall")
	}
	if isLoginWall(`<meta property="og:title" content="Top preto" /><meta property="og:image" content="https://x" />`) {
		t.Fatal("listing preview should not be a login wall")
	}
}

func TestFetchPublicReel(t *testing.T) {
	if os.Getenv("FB_LIVE") == "" {
		t.Skip("set FB_LIVE=1 to run a live facebook fetch")
	}
	resp, err := Fetch("https://www.facebook.com/reel/730293269054758")
	if err != nil {
		t.Fatal(err)
	}
	if resp.AuthorName == "" {
		t.Fatal("expected author name")
	}
	if len(resp.MediaDetails) == 0 || resp.MediaDetails[0].Type != "video" {
		t.Fatalf("expected video, got %+v", resp.MediaDetails)
	}
	if resp.Caption == "" {
		t.Fatal("expected caption")
	}
}
