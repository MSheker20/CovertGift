package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/genai"
)

const (
	DefaultResearchModel  = "gemini-3.8-flash"
	DefaultSelectionModel = "gemini-3.5-flash-lite"
)

type fakeTransport func(*http.Request) (*http.Response, error)

func (f fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testClient(t *testing.T, handler fakeTransport) *Client {
	t.Helper()
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: "test-key", Backend: genai.BackendGeminiAPI,
		HTTPClient: &http.Client{Transport: handler},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Client{client: client, researchModel: DefaultResearchModel, model: DefaultSelectionModel, searchEnabled: true, fallbackModel: DefaultSelectionModel}
}

func jsonResponse(t *testing.T, value any) *http.Response {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(data)))}
}

func textResponse(text string, metadata *genai.GroundingMetadata) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{
		Content: &genai.Content{Parts: []*genai.Part{{Text: text}}}, GroundingMetadata: metadata,
	}}}
}

func researchResponse() *genai.GenerateContentResponse {
	text := "AeroPress Go Kahve Seti Trendyol Türkiye mağazasında 2.499,90 TL fiyatla satışta. Kahve merakı ve kamp için uygundur."
	return textResponse(text, &genai.GroundingMetadata{
		WebSearchQueries: []string{"site:trendyol.com AeroPress Go"},
		GroundingChunks: []*genai.GroundingChunk{{Web: &genai.GroundingChunkWeb{
			URI: "https://vertexaisearch.cloud.google.com/grounding-api-redirect/test", Title: "trendyol.com",
		}}},
		GroundingSupports: []*genai.GroundingSupport{{Segment: &genai.Segment{Text: text}, GroundingChunkIndices: []int32{0}}},
		SearchEntryPoint:  &genai.SearchEntryPoint{RenderedContent: "<div>Google önerileri</div>"},
	})
}

const selectedProduct = `[{"title":"AeroPress Go Kahve Seti","description":"Kamp sırasında kahve demlemeyi seven kişi için taşınabilir ekipman.","estimated_price":2499.90,"search_query":"AeroPress Go Kahve Seti","evidence_ids":[1]}]`

func TestResearchThenSelectGroundedProduct(t *testing.T) {
	calls := 0
	client := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if calls == 1 {
			if !strings.Contains(r.URL.Path, "gemini-3.8-flash:") {
				t.Fatalf("research used the wrong model: %s", r.URL.Path)
			}
			if body["tools"] == nil {
				t.Fatal("research must enable search")
			}
			return jsonResponse(t, researchResponse()), nil
		}
		if !strings.Contains(r.URL.Path, "gemini-3.5-flash-lite:") || body["tools"] != nil || body["generationConfig"] == nil {
			t.Fatal("selection must use structured output separately from search")
		}
		data, _ := json.Marshal(body)
		if !strings.Contains(string(data), "2.499,90") {
			t.Fatal("selection did not receive supported research")
		}
		return jsonResponse(t, textResponse(selectedProduct, nil)), nil
	})
	got, err := client.GenerateGiftIdeas(context.Background(), "Kahve, kamp; 3000 TL")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(got) != 1 || len(got[0].Sources) != 1 || got[0].Sources[0].Title != "trendyol.com" || got[0].SearchSuggestions == "" {
		t.Fatalf("unexpected grounded selection: %+v (%d calls)", got, calls)
	}
}

func TestRejectUnsupportedResearchWithoutFormatting(t *testing.T) {
	for _, tc := range []string{"no metadata", "no search", "no citation", "foreign shop", "spoofed shop", "bad citation index", "unsafe source"} {
		t.Run(tc, func(t *testing.T) {
			response := researchResponse()
			metadata := response.Candidates[0].GroundingMetadata
			switch tc {
			case "no metadata":
				response.Candidates[0].GroundingMetadata = nil
			case "no search":
				metadata.WebSearchQueries = nil
			case "no citation":
				metadata.GroundingSupports = nil
			case "foreign shop":
				metadata.GroundingChunks[0].Web.URI = "https://www.amazon.com/dp/test"
			case "spoofed shop":
				metadata.GroundingChunks[0].Web.URI = "https://trendyol.com.evil.example/test"
			case "bad citation index":
				metadata.GroundingSupports[0].GroundingChunkIndices = []int32{-1, 20}
			case "unsafe source":
				metadata.GroundingChunks[0].Web.URI = "javascript:alert(1)"
			}
			calls := 0
			client := testClient(t, func(*http.Request) (*http.Response, error) {
				calls++
				return jsonResponse(t, response), nil
			})
			_, err := client.GenerateGiftIdeas(context.Background(), "Kahve")
			if !errors.Is(err, ErrNoGroundedProducts) || calls != 1 {
				t.Fatalf("err = %v, calls = %d", err, calls)
			}
		})
	}
}

func TestRejectInventedAndIncompleteSelections(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"invalid evidence", strings.Replace(selectedProduct, "[1]", "[99]", 1)},
		{"missing evidence", strings.Replace(selectedProduct, "[1]", "[]", 1)},
		{"invented model", strings.ReplaceAll(selectedProduct, "AeroPress Go Kahve Seti", "Hayali Kahve Robotu")},
		{"invented price", strings.Replace(selectedProduct, "2499.90", "1500", 1)},
		{"zero price", strings.Replace(selectedProduct, "2499.90", "0", 1)},
		{"negative price", strings.Replace(selectedProduct, "2499.90", "-100", 1)},
		{"database overflow", strings.Replace(selectedProduct, "2499.90", "100000000", 1)},
		{"empty description", strings.Replace(selectedProduct, "Kamp sırasında kahve demlemeyi seven kişi için taşınabilir ekipman.", "", 1)},
		{"query is URL", strings.Replace(selectedProduct, `"search_query":"AeroPress Go Kahve Seti"`, `"search_query":"https://trendyol.com/test"`, 1)},
		{"empty array", "[]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := testClient(t, func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return jsonResponse(t, researchResponse()), nil
				}
				return jsonResponse(t, textResponse(tc.text, nil)), nil
			})
			got, err := client.GenerateGiftIdeas(context.Background(), "Kahve")
			if !errors.Is(err, ErrNoGroundedProducts) || len(got) != 0 {
				t.Fatalf("got = %v, err = %v", got, err)
			}
		})
	}
}

func Test503FallbackKeepsSearchEnabled(t *testing.T) {
	calls := 0
	client := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 503, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":503,"message":"busy","status":"UNAVAILABLE"}}`))}, nil
		}
		if calls == 2 {
			data, _ := io.ReadAll(r.Body)
			if !strings.Contains(r.URL.Path, "gemini-3.5-flash-lite") || !strings.Contains(string(data), "googleSearch") {
				t.Fatalf("fallback lost grounding: %s %s", r.URL.Path, data)
			}
			return jsonResponse(t, researchResponse()), nil
		}
		return jsonResponse(t, textResponse(selectedProduct, nil)), nil
	})
	if _, err := client.GenerateGiftIdeas(context.Background(), "Kahve"); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d", calls)
	}
}

func Test429StopsWithoutFallbackAndPreservesQuotaError(t *testing.T) {
	for _, stage := range []string{"research", "selection"} {
		t.Run(stage, func(t *testing.T) {
			calls := 0
			client := testClient(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if stage == "selection" && calls == 1 {
					return jsonResponse(t, researchResponse()), nil
				}
				return &http.Response{StatusCode: 429, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":429,"message":"quota exhausted","status":"RESOURCE_EXHAUSTED"}}`))}, nil
			})
			_, err := client.GenerateGiftIdeas(context.Background(), "Kahve")
			var upstream genai.APIError
			if !errors.Is(err, ErrRateLimited) || !errors.As(err, &upstream) || upstream.Code != 429 {
				t.Fatalf("lost quota error: %v", err)
			}
			wantCalls := 1
			if stage == "selection" {
				wantCalls = 2
			}
			if calls != wantCalls {
				t.Fatalf("quota exhaustion caused extra calls: got %d, want %d", calls, wantCalls)
			}
		})
	}
}

func TestSelection503FallbackDoesNotEnableSearch(t *testing.T) {
	calls := 0
	client := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return jsonResponse(t, researchResponse()), nil
		}
		if calls == 2 {
			return &http.Response{StatusCode: 503, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":503,"message":"busy","status":"UNAVAILABLE"}}`))}, nil
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(r.URL.Path, DefaultSelectionModel) || body["tools"] != nil || body["generationConfig"] == nil {
			t.Fatalf("selection fallback lost its model/schema or enabled Search: %s %+v", r.URL.Path, body)
		}
		return jsonResponse(t, textResponse(selectedProduct, nil)), nil
	})
	client.model = "gemini-3.8-flash"
	if _, err := client.GenerateGiftIdeas(context.Background(), "Kahve"); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestGroundingOffsetsUseUTF8Bytes(t *testing.T) {
	response := researchResponse()
	candidate := response.Candidates[0]
	support := candidate.GroundingMetadata.GroundingSupports[0]
	support.Segment.Text = ""
	support.Segment.EndIndex = int32(len(candidate.Content.Parts[0].Text))
	evidence, _ := groundedEvidence(response)
	if len(evidence) != 1 || evidence[0].Text != candidate.Content.Parts[0].Text {
		t.Fatalf("lost Turkish text: %+v", evidence)
	}
}

func TestSearchDisabledUsesConfiguredModelAndDiscardsEvidence(t *testing.T) {
	calls := 0
	client := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(r.URL.Path, "custom-model:") || body["tools"] != nil || body["generationConfig"] == nil {
			t.Fatalf("unexpected request: %s %+v", r.URL.Path, body)
		}
		// Repeated, invalid and fabricated evidence must not reach the service.
		items := strings.TrimSuffix(selectedProduct, "]") + "," + strings.TrimPrefix(selectedProduct, "[")
		return jsonResponse(t, textResponse(items, researchResponse().Candidates[0].GroundingMetadata)), nil
	})
	client.model, client.searchEnabled = "custom-model", false
	got, err := client.GenerateGiftIdeas(context.Background(), "Kahve")
	if err != nil || calls != 1 || len(got) != 1 {
		t.Fatalf("got=%+v err=%v calls=%d", got, err, calls)
	}
	if !got[0].Unverified || len(got[0].Sources) != 0 || len(got[0].EvidenceIDs) != 0 || got[0].SearchSuggestions != "" {
		t.Fatalf("fabricated verification: %+v", got[0])
	}
}

func TestSearchDisabledRejectsInvalidPrice(t *testing.T) {
	client := testClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(t, textResponse(strings.Replace(selectedProduct, "2499.90", "-1", 1), nil)), nil
	})
	client.searchEnabled = false
	if _, err := client.GenerateGiftIdeas(context.Background(), "Kahve"); !errors.Is(err, ErrEmptyResponse) {
		t.Fatalf("err=%v", err)
	}
}

func TestFallbackIsOptionalAndKeepsSearchDisabled(t *testing.T) {
	for _, fallback := range []string{"", "custom-primary", "custom-backup"} {
		t.Run(fallback, func(t *testing.T) {
			calls := 0
			client := testClient(t, func(r *http.Request) (*http.Response, error) {
				calls++
				data, _ := io.ReadAll(r.Body)
				if strings.Contains(string(data), "googleSearch") {
					t.Fatal("fallback enabled Search")
				}
				if calls == 1 {
					return &http.Response{StatusCode: 503, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":503,"message":"busy","status":"UNAVAILABLE"}}`))}, nil
				}
				if !strings.Contains(r.URL.Path, "custom-backup:") {
					t.Fatalf("wrong fallback: %s", r.URL.Path)
				}
				return jsonResponse(t, textResponse(selectedProduct, nil)), nil
			})
			client.model, client.searchEnabled, client.fallbackModel = "custom-primary", false, fallback
			_, err := client.GenerateGiftIdeas(context.Background(), "Kahve")
			if fallback == "custom-backup" {
				if err != nil || calls != 2 {
					t.Fatalf("err=%v calls=%d", err, calls)
				}
			} else if err == nil || calls != 1 {
				t.Fatalf("unexpected retry: err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestModelMustBeConfigured(t *testing.T) {
	if _, err := NewClientWithOptions("test-key", Options{}); err == nil {
		t.Fatal("missing model accepted")
	}
	client, err := NewClientWithOptions("test-key", Options{Model: " custom-model "})
	if err != nil {
		t.Fatal(err)
	}
	if client.model != "custom-model" || client.researchModel != "custom-model" || client.searchEnabled || client.fallbackModel != "" {
		t.Fatalf("unexpected configuration: %+v", client)
	}
}

func TestGenerateWrapsPersistent503AsOverloaded(t *testing.T) {
	calls := 0
	client := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":503,"message":"high demand","status":"UNAVAILABLE"}}`))}, nil
	})
	client.fallbackModel = "yedek-model"
	_, err := client.generate(context.Background(), "birincil-model", "Kahve", nil)
	var upstream genai.APIError
	if !errors.Is(err, ErrOverloaded) || !errors.As(err, &upstream) || upstream.Code != 503 {
		t.Fatalf("503 not mapped to ErrOverloaded: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected primary + fallback attempt, got %d calls", calls)
	}
}
