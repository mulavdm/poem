package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mulavdm/poem/pkg/app"
)

func mapResourceHandler[S any](application app.App[S]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if application.MapResources == nil {
			http.NotFound(w, r)
			return
		}
		session := sessionFromContext[S](r.Context())
		state, _ := session.Snapshot()
		providerID := r.URL.Query().Get("provider")
		sourceID := r.URL.Query().Get("source")
		snapshot := r.URL.Query().Get("snapshot")
		mapNode, sourcePresent := currentMapSource(app.ResolvedView(application, state), providerID, sourceID, snapshot)
		if !sourcePresent {
			http.Error(w, "invalid map source", http.StatusBadRequest)
			return
		}
		providers := application.MapResources(state)
		var provider app.MapResourceProvider
		seen := make(map[string]bool, len(providers))
		for _, candidate := range providers {
			if !candidate.Valid() || seen[candidate.ID] {
				http.Error(w, "invalid map provider registry", http.StatusInternalServerError)
				return
			}
			seen[candidate.ID] = true
			if candidate.ID == providerID {
				provider = candidate
			}
		}
		if !provider.Valid() {
			http.Error(w, "invalid map provider", http.StatusBadRequest)
			return
		}
		kindValue, err := strconv.Atoi(r.URL.Query().Get("kind"))
		if err != nil {
			http.Error(w, "invalid resource kind", http.StatusBadRequest)
			return
		}
		request := app.MapResourceRequest{SourceID: sourceID, Snapshot: snapshot, Kind: app.MapResourceKind(kindValue), Name: r.URL.Query().Get("name")}
		if request.Kind == app.MapResourceVectorTile || request.Kind == app.MapResourceElevationTile {
			request.Z, err = queryInt(r, "z")
			if err == nil {
				request.X, err = queryInt(r, "x")
			}
			if err == nil {
				request.Y, err = queryInt(r, "y")
			}
		}
		if err != nil || !request.Valid() {
			http.Error(w, "invalid map resource request", http.StatusBadRequest)
			return
		}
		resource, err := provider.Fetch(r.Context(), request)
		if err != nil {
			http.Error(w, "map resource unavailable", http.StatusBadGateway)
			return
		}
		if !resource.Valid() || strings.ContainsAny(resource.ContentType, "\r\n") || strings.ContainsAny(resource.ETag, "\r\n") {
			http.Error(w, "invalid map resource", http.StatusBadGateway)
			return
		}
		contentType := resource.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if resource.ETag != "" {
			w.Header().Set("ETag", resource.ETag)
			if r.Header.Get("If-None-Match") == resource.ETag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		if mapNode.CachePolicy == app.MapCacheMemoryOnly {
			w.Header().Set("Cache-Control", "private, no-store")
		} else if resource.Immutable {
			w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "private, no-cache")
		}
		_, _ = w.Write(resource.Bytes)
	}
}

func queryInt(r *http.Request, name string) (int, error) {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return value, nil
}

func currentMapSource(node app.Node, providerID, sourceID, snapshot string) (app.MapViewportNode, bool) {
	var found app.MapViewportNode
	present := false
	var visit func(app.Node)
	visitNodes := func(nodes []app.Node) {
		for _, child := range nodes {
			visit(child)
		}
	}
	visit = func(node app.Node) {
		if present {
			return
		}
		switch n := node.(type) {
		case app.MapViewportNode:
			if n.Source.ProviderID == providerID && n.Source.ID == sourceID && n.Source.Snapshot == snapshot && n.Source.Valid() {
				found, present = n, true
			}
		case app.IdentityNode:
			visit(n.Child)
		case app.CollectionNode:
			for _, item := range n.Items {
				for _, action := range item.Actions {
					visit(action)
				}
			}
		case app.SectionNode:
			visitNodes(n.Children)
		case app.WorkspaceNode:
			visitNodes(n.Header)
			if n.Navigation != nil {
				visit(n.Navigation)
			}
			if n.Status != nil {
				visit(n.Status)
			}
			visitNodes(n.Content)
			visitNodes(n.Tools)
			visitNodes(n.Detail)
		case app.ContainerNode:
			visitNodes(n.Children)
		case app.ResponsiveNode:
			visitNodes(n.Compact)
			visitNodes(n.Wide)
		case app.TabsNode:
			if active := n.ActiveIndex(); active >= 0 {
				visitNodes(n.Tabs[active].Content)
			}
		case app.AccordionNode:
			for _, section := range n.Sections {
				visitNodes(section.Content)
			}
		case app.ModalNode:
			visitNodes(n.Content)
		case app.OverlayNode:
			if n.Base != nil {
				visit(n.Base)
			}
			for _, layer := range n.Layers {
				if layer.Content != nil {
					visit(layer.Content)
				}
			}
		}
	}
	visit(node)
	return found, present
}
