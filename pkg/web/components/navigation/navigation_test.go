package navigation

import (
	"strconv"
	"strings"
	"testing"
)

func TestPaginationSmallTotalShowsEveryPageWithoutWindowing(t *testing.T) {
	html := string(Pagination{Current: 2, Total: 5, BaseURL: "/items"}.HTML())
	for page := 1; page <= 5; page++ {
		if !strings.Contains(html, ">"+strconv.Itoa(page)+"</a>") {
			t.Fatalf("missing page %d link in %q", page, html)
		}
	}
	if strings.Contains(html, "poem-pagination__ellipsis") {
		t.Fatalf("unexpected ellipsis for small total: %q", html)
	}
}

func TestPaginationLargeTotalWithCurrentInMiddleShowsBothEllipses(t *testing.T) {
	html := string(Pagination{Current: 25, Total: 50, BaseURL: "/items", MaxButtons: 7}.HTML())
	if got := strings.Count(html, "poem-pagination__ellipsis"); got != 2 {
		t.Fatalf("expected 2 ellipses, got %d in %q", got, html)
	}
	for _, want := range []string{">1</a>", ">50</a>", `>25</a>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %q", want, html)
		}
	}
	if !strings.Contains(html, `aria-current="page">25</a>`) {
		t.Fatalf("current page not marked: %q", html)
	}
}

func TestPaginationCurrentNearEdgeShowsOnlyOneEllipsis(t *testing.T) {
	html := string(Pagination{Current: 2, Total: 50, BaseURL: "/items", MaxButtons: 7}.HTML())
	if got := strings.Count(html, "poem-pagination__ellipsis"); got != 1 {
		t.Fatalf("expected 1 ellipsis near the start edge, got %d in %q", got, html)
	}
	for page := 1; page <= 7; page++ {
		if !strings.Contains(html, ">"+strconv.Itoa(page)+"</a>") {
			t.Fatalf("missing page %d link in %q", page, html)
		}
	}
	if !strings.Contains(html, ">50</a>") {
		t.Fatalf("missing last page link in %q", html)
	}

	html = string(Pagination{Current: 49, Total: 50, BaseURL: "/items", MaxButtons: 7}.HTML())
	if got := strings.Count(html, "poem-pagination__ellipsis"); got != 1 {
		t.Fatalf("expected 1 ellipsis near the end edge, got %d in %q", got, html)
	}
	if !strings.Contains(html, ">1</a>") {
		t.Fatalf("missing first page link in %q", html)
	}
}

func TestPaginationZeroTotalRendersNothing(t *testing.T) {
	pagination := Pagination{Current: 1, Total: 0}
	if html := pagination.HTML(); html != "" {
		t.Fatalf("expected empty output, got %q", html)
	}
}
