// Package poem drives a Trellis app.App as a real running POEM desktop
// application.
package desktop

import (
	"fmt"
	"image"
	"math"

	"github.com/mulavdm/poem/pkg/render"

	"github.com/mulavdm/poem/pkg/app"
)

// build converts a app.Node tree into a real POEM component tree. path is a
// stable, deterministic component ID for this node — derived from its
// position in the tree, not from any node identity POEM would otherwise
// track — since the whole tree is rebuilt from scratch on every call, the
// same way POEM's own BuildPagesFn convention already works.
func build(node app.Node, path string, dispatch func(app.Msg)) render.Component {
	switch n := node.(type) {
	case app.IdentityNode:
		return build(n.Child, semanticPath(path, n.Semantic.ID), dispatch)

	case app.ActionNode:
		button := render.NewButton(path, desktopActionLabel(n.Icon, n.Label), func(*render.ApplicationState) { dispatch(n.Invoke) })
		button.Disabled = !n.Semantic.Enabled || n.Semantic.Running
		button.Loading = n.Semantic.Running
		button.Selected = n.Selected
		switch n.Importance {
		case app.ImportancePrimary:
			button.Variant = render.VariantPrimary
		case app.ImportanceSubtle:
			button.Variant = render.VariantSubtle
		default:
			button.Variant = render.VariantSecondary
		}
		return button

	case app.TextNode:
		return render.NewLabel(path, n.Value)

	case app.ButtonNode:
		button := render.NewButton(path, desktopActionLabel(n.Icon, n.Label), func(*render.ApplicationState) {
			dispatch(n.OnClick)
		})
		button.Disabled = n.Disabled
		button.Variant = poemVariant(n.Variant)
		return button

	case app.TextInputNode:
		input := render.NewTextInput(path, n.Placeholder)
		input.Value = n.Value
		input.AccessibleName = n.Label
		input.Disabled = n.Disabled
		input.ReadOnly = n.ReadOnly
		input.Invalid = n.Error != ""
		msg := n.OnChange
		input.OnChange = func(value string, _ *render.ApplicationState) {
			dispatch(app.Msg{Name: msg.Name, Payload: value})
		}
		return input

	case app.TextAreaNode:
		area := render.NewTextArea(path, n.Placeholder)
		area.Value = n.Value
		area.AccessibleName = n.Label
		area.Disabled = n.Disabled
		msg := n.OnChange
		area.OnChange = func(value string, _ *render.ApplicationState) {
			dispatch(app.Msg{Name: msg.Name, Payload: value})
		}
		return area

	case app.SliderNode:
		msg := n.OnChange
		slider := render.NewSlider(path, float32(n.Min), float32(n.Max), float32(n.Value), func(value float32, _ *render.ApplicationState) {
			dispatch(app.Msg{Name: msg.Name, Payload: app.FloatPayload(float64(value))})
		})
		slider.Disabled = n.Disabled
		slider.AccessibleName = n.Label
		return slider

	case app.BadgeNode:
		badge := render.NewBadge(path, n.Text)
		badge.Variant = poemVariant(n.Variant)
		return badge

	case app.ProgressBarNode:
		bar := render.NewProgressBar(path)
		bar.Min = 0
		if n.Max > 0 {
			bar.Max = float32(n.Max)
		}
		bar.Value = float32(n.Value)
		bar.Indeterminate = n.Indeterminate
		bar.AccessibleName = n.Label
		return bar

	case app.CheckboxNode:
		msg := n.OnChange
		checkbox := render.NewCheckbox(path, n.Label, n.Checked, func(checked bool, _ *render.ApplicationState) {
			dispatch(app.Msg{Name: msg.Name, Payload: app.BoolPayload(checked)})
		})
		checkbox.Disabled = n.Disabled
		return checkbox

	case app.SwitchNode:
		msg := n.OnChange
		toggle := render.NewSwitch(path, n.Label, n.Checked, func(checked bool, _ *render.ApplicationState) {
			dispatch(app.Msg{Name: msg.Name, Payload: app.BoolPayload(checked)})
		})
		toggle.Disabled = n.Disabled
		return toggle

	case app.SelectNode:
		options := make([]render.SelectOption, len(n.Options))
		for i, option := range n.Options {
			options[i] = render.SelectOption{Value: option.Value, Label: option.Label}
		}
		msg := n.OnChange
		selectBox := render.NewSelect(path, options, n.Value, func(value string, _ *render.ApplicationState) {
			dispatch(app.Msg{Name: msg.Name, Payload: value})
		})
		selectBox.Placeholder = n.Placeholder
		selectBox.Disabled = n.Disabled
		selectBox.AccessibleName = n.Label
		return selectBox

	case app.RadioGroupNode:
		msg := n.OnChange
		onSelect := func(value string, _ *render.ApplicationState) {
			dispatch(app.Msg{Name: msg.Name, Payload: value})
		}
		radios := make([]render.Component, len(n.Options))
		for i, option := range n.Options {
			radio := render.NewRadio(childPath(path, i), option.Label, option.Value, option.Value == n.Value, onSelect)
			radio.Disabled = n.Disabled
			radios[i] = radio
		}
		return &render.FlexBox{
			CompID:    path,
			Direction: render.Vertical,
			Gap:       4,
			Children:  radios,
		}

	case app.ImageNode:
		view := &render.ImageView{CompID: path}
		if decoded, ok := decodeImageCached(n.Encoded); ok {
			view.ImageWidth = decoded.width
			view.ImageHeight = decoded.height
			view.Pixels = decoded.pixels
		}
		if n.MaxWidth > 0 || n.MaxHeight > 0 {
			// An explicit rect bounds the layout; ImageView keeps the aspect
			// ratio inside it via its contain-fit measure.
			view.Rect = image.Rect(0, 0, n.MaxWidth, n.MaxHeight)
		}
		return view

	case app.ImageViewportNode:
		view := &render.ImageViewport{
			CompID: path,
			Alt:    n.Image.Alt,
			Transform: render.ImageTransform{
				OffsetX: n.Transform.OffsetX,
				OffsetY: n.Transform.OffsetY,
				Scale:   n.Transform.Normalized().Scale,
			},
			MinScale: n.MinScale,
			MaxScale: n.MaxScale,
			Disabled: n.Disabled,
		}
		if decoded, ok := decodeImageCached(n.Image.Encoded); ok {
			view.ImageWidth = decoded.width
			view.ImageHeight = decoded.height
			view.Pixels = decoded.pixels
		}
		view.MaxWidth, view.MaxHeight = n.Image.MaxWidth, n.Image.MaxHeight
		msg := n.OnChange
		if msg.Name != "" {
			view.OnChange = func(transform render.ImageTransform, _ *render.ApplicationState) {
				bounds := view.Bounds()
				dispatch(app.Msg{Name: msg.Name, Payload: app.ImageViewportCommitPayload(app.ImageViewportCommit{
					OffsetX: transform.OffsetX,
					OffsetY: transform.OffsetY,
					Scale:   transform.Scale,
					Width:   bounds.Dx(), Height: bounds.Dy(),
				})})
			}
		}
		activate := n.OnActivate
		if activate.Name != "" {
			view.OnActivate = func(x, y float64, _ *render.ApplicationState) {
				dispatch(app.Msg{Name: activate.Name, Payload: app.ViewportPointPayload(app.ViewportPoint{X: x, Y: y})})
			}
		}
		view.Markers = make([]render.ImageMarker, 0, len(n.Markers))
		markerMessages := make(map[string]app.Msg, len(n.Markers))
		for _, marker := range n.Markers {
			if marker.ID == "" || marker.X < 0 || marker.X > 1 || marker.Y < 0 || marker.Y > 1 {
				continue
			}
			view.Markers = append(view.Markers, render.ImageMarker{ID: marker.ID, Label: marker.Label, X: marker.X, Y: marker.Y,
				Variant: imageMarkerVariant(marker.Variant), Selected: marker.Selected, Disabled: marker.Disabled})
			if marker.OnActivate.Name != "" {
				markerMessages[marker.ID] = marker.OnActivate
			}
		}
		if len(markerMessages) > 0 {
			view.OnMarker = func(id string, _ *render.ApplicationState) {
				if markerMsg, ok := markerMessages[id]; ok {
					markerMsg.Payload = id
					dispatch(markerMsg)
				}
			}
		}
		return view

	case app.MapViewportNode:
		if !n.Valid() {
			return render.NewLabel(path+"/invalid", "Map unavailable")
		}
		// A map viewport fills its container and re-fills on every resize; the
		// fallback's Max dimensions are the raster resolution, not a display cap,
		// so they are deliberately not imposed on the interactive canvas here.
		view := &render.ImageViewport{
			CompID: path, Alt: n.Semantic.Name, MinScale: 0.5, MaxScale: 8,
			Disabled: !n.Semantic.Enabled, MapViewportID: n.Semantic.ID,
			MapInteraction: true, MinPitch: n.MinPitch, MaxPitch: n.MaxPitch,
			Transform: render.ImageTransform{Scale: 1, Bearing: n.Camera.Bearing, Pitch: n.Camera.Pitch},
		}
		if decoded, ok := decodeImageCached(n.Fallback.Encoded); ok {
			view.ImageWidth, view.ImageHeight, view.Pixels = decoded.width, decoded.height, decoded.pixels
		}
		if n.OnCameraChange.Name != "" {
			msg := n.OnCameraChange
			camera := n.Camera.Normalized()
			view.OnChange = func(transform render.ImageTransform, _ *render.ApplicationState) {
				next := camera
				next.Zoom = math.Max(n.MinZoom, math.Min(n.MaxZoom, camera.Zoom+math.Log2(transform.Scale)))
				world := math.Exp2(camera.Zoom)
				next.Longitude = math.Max(-180, math.Min(180, camera.Longitude-transform.OffsetX*360/world))
				next.Latitude = math.Max(-85.05112878, math.Min(85.05112878, camera.Latitude+transform.OffsetY*170/world))
				next.Bearing = transform.Bearing
				next.Pitch = math.Max(n.MinPitch, math.Min(n.MaxPitch, transform.Pitch))
				dispatch(app.Msg{Name: msg.Name, Payload: app.MapCameraPayload(next)})
			}
		}
		bounds := n.Source.Bounds
		featureMessages := make(map[string]app.Msg)
		if bounds.Valid() {
			for _, feature := range n.Features {
				if !feature.Valid() || feature.Geometry != app.MapGeometryPoint {
					continue
				}
				position := feature.Positions[0]
				x := (position.Longitude - bounds.West) / (bounds.East - bounds.West)
				y := (bounds.North - position.Latitude) / (bounds.North - bounds.South)
				if x < 0 || x > 1 || y < 0 || y > 1 {
					continue
				}
				view.Markers = append(view.Markers, render.ImageMarker{ID: feature.ID, Label: feature.Name, X: x, Y: y, Selected: feature.Selected, Disabled: feature.Disabled})
				msg := feature.OnActivate
				if msg.Name == "" {
					msg = n.OnFeature
				}
				if msg.Name != "" {
					featureMessages[feature.ID] = msg
				}
			}
		}
		if len(featureMessages) > 0 {
			view.OnMarker = func(id string, _ *render.ApplicationState) {
				msg := featureMessages[id]
				msg.Payload = id
				dispatch(msg)
			}
		}
		return view

	case app.TableNode:
		columns := make([]render.TableColumn, len(n.Columns))
		for i, column := range n.Columns {
			columns[i] = render.TableColumn{Key: column.Key, Label: column.Label}
		}
		rows := make([]render.TableRow, len(n.Rows))
		for i, row := range n.Rows {
			rows[i] = render.TableRow{ID: fmt.Sprintf("%s/row-%d", path, i), Values: row.Cells}
		}
		return render.NewDataTable(path, columns, rows)

	case app.CollectionNode:
		items := make([]render.Component, 0, len(n.Items))
		for itemIndex, item := range n.Items {
			children := []render.Component{render.NewLabel(childPath(path, itemIndex)+"/title", item.Title)}
			if item.Description != "" {
				children = append(children, render.NewLabel(childPath(path, itemIndex)+"/description", item.Description))
			}
			for fieldIndex, field := range item.Metadata {
				children = append(children, render.NewLabel(fmt.Sprintf("%s/meta-%d", childPath(path, itemIndex), fieldIndex), field.Label+": "+field.Value))
			}
			for actionIndex, action := range item.Actions {
				children = append(children, build(action, fmt.Sprintf("%s/action-%d", childPath(path, itemIndex), actionIndex), dispatch))
			}
			items = append(items, &render.FlexBox{CompID: childPath(path, itemIndex), Direction: render.Vertical, Gap: 6, Padding: 12, Children: children})
		}
		return &render.FlexBox{CompID: path, Direction: render.Vertical, Gap: 8, Children: items}

	case app.SectionNode:
		children := []render.Component{render.NewLabel(path+"/title", n.Title)}
		if n.Description != "" {
			children = append(children, render.NewLabel(path+"/description", n.Description))
		}
		for index, child := range n.Children {
			children = append(children, build(child, childPath(path, index), dispatch))
		}
		return &render.FlexBox{CompID: path, Direction: render.Vertical, Gap: 8, Padding: 12, Children: children}

	case app.WorkspaceNode:
		titleChildren := []render.Component{render.NewLabel(path+"/title", n.Title)}
		if n.Subtitle != "" {
			titleChildren = append(titleChildren, render.NewLabel(path+"/subtitle", n.Subtitle))
		}
		headerChildren := []render.Component{&render.FlexBox{CompID: path + "/heading", Direction: render.Vertical, Gap: 2, Children: titleChildren}}
		if n.Status != nil {
			headerChildren = append(headerChildren, build(n.Status, path+"/status", dispatch))
		}
		for index, child := range n.Header {
			headerChildren = append(headerChildren, build(child, childPath(path+"/header", index), dispatch))
		}
		header := &render.FlexBox{CompID: path + "/header", Direction: render.Horizontal, Wrap: true, Gap: 10, LineGap: 8, Padding: 12, Children: headerChildren}
		contentChildren := make([]render.Component, len(n.Content))
		for index, child := range n.Content {
			contentChildren[index] = build(child, childPath(path+"/content", index), dispatch)
		}
		toolChildren := make([]render.Component, len(n.Tools))
		for index, child := range n.Tools {
			toolChildren[index] = build(child, childPath(path+"/tools", index), dispatch)
		}
		detailChildren := make([]render.Component, len(n.Detail))
		for index, child := range n.Detail {
			detailChildren[index] = build(child, childPath(path+"/detail", index), dispatch)
		}
		content := &render.FlexBox{CompID: path + "/content", Direction: render.Vertical, Gap: 8, Children: contentChildren}
		tools := &render.FlexBox{CompID: path + "/tools", Direction: render.Vertical, Gap: 10, Padding: 16, Children: toolChildren}
		toolsViewport := render.NewScrollView(path+"/tools-scroll", tools)
		var detailViewport render.Component
		if len(detailChildren) > 0 {
			detail := &render.FlexBox{CompID: path + "/detail", Direction: render.Vertical, Gap: 10, Padding: 16, Children: detailChildren}
			detailViewport = render.NewScrollView(path+"/detail-scroll", detail)
		}
		return &render.Workspace{CompID: path, Header: header, Content: content, Tools: toolsViewport, Detail: detailViewport}

	case app.ResponsiveNode:
		compact := make([]render.Component, len(n.Compact))
		for index, child := range n.Compact {
			compact[index] = build(child, childPath(path+"/compact", index), dispatch)
		}
		wide := make([]render.Component, len(n.Wide))
		for index, child := range n.Wide {
			wide[index] = build(child, childPath(path+"/wide", index), dispatch)
		}
		breakpoint := n.Breakpoint
		if breakpoint <= 0 {
			breakpoint = 600
		}
		return &render.Responsive{CompID: path, Breakpoint: breakpoint,
			Compact: &render.FlexBox{CompID: path + "/compact", Direction: render.Vertical, Gap: 6, Children: compact},
			Wide:    &render.FlexBox{CompID: path + "/wide", Direction: render.Vertical, Gap: 6, Children: wide}}

	case app.TabsNode:
		items := make([]render.TabItem, len(n.Tabs))
		for i, tab := range n.Tabs {
			items[i] = render.TabItem{ID: tab.ID, Label: tab.Label}
		}
		msg := n.OnChange
		// Controlled: OnChange is set, so POEM treats App[S] as the owner of the
		// selection rather than self-managing it (see app.TabsNode).
		strip := render.NewTabs(path+"/strip", items, n.Selected, func(id string, _ *render.ApplicationState) {
			dispatch(app.Msg{Name: msg.Name, Payload: id})
		})
		children := []render.Component{strip}
		if active := n.ActiveIndex(); active >= 0 {
			tabPath := childPath(path, active)
			content := make([]render.Component, len(n.Tabs[active].Content))
			for j, child := range n.Tabs[active].Content {
				content[j] = build(child, childPath(tabPath, j), dispatch)
			}
			children = append(children, &render.FlexBox{
				CompID:    tabPath + "/panel",
				Direction: render.Vertical,
				Gap:       8,
				Children:  content,
			})
		}
		return &render.FlexBox{
			CompID:    path,
			Direction: render.Vertical,
			Gap:       8,
			Children:  children,
		}

	case app.AccordionNode:
		items := make([]render.AccordionItem, len(n.Sections))
		expanded := map[string]bool{}
		for i, section := range n.Sections {
			sectionID := fmt.Sprintf("s%d", i)
			sectionPath := childPath(path, i)
			content := make([]render.Component, len(section.Content))
			for j, child := range section.Content {
				content[j] = build(child, childPath(sectionPath, j), dispatch)
			}
			items[i] = render.AccordionItem{
				ID:    sectionID,
				Title: section.Title,
				Content: &render.FlexBox{
					CompID:    sectionPath + "/body",
					Direction: render.Vertical,
					Gap:       8,
					Children:  content,
				},
			}
			if section.DefaultOpen {
				expanded[sectionID] = true
			}
		}
		// nil OnToggle: the Accordion self-manages expansion in POEM's transient
		// store, so open/closed state stays backend-local chrome and never
		// enters App[S] — matching how the web backend's <details> toggles
		// client-side. See app.AccordionNode.
		return render.NewAccordion(path, items, expanded, nil)

	case app.ContainerNode:
		children := make([]render.Component, len(n.Children))
		for i, child := range n.Children {
			children[i] = build(child, childPath(path, i), dispatch)
		}
		return &render.FlexBox{
			CompID:    path,
			Direction: flexDirection(n.Direction),
			Gap:       n.Gap,
			Padding:   n.Padding,
			// Wrap is deliberately off by default. Enabling it for every
			// horizontal row changed layout semantics app-wide and exposed a
			// measure-vs-layout height inconsistency (a wrapped row measured
			// shorter than it laid out, so the next sibling overlapped it). The
			// truncation this was meant to solve is fixed at the source instead
			// — controls now size to their content — so rows that fit no longer
			// need wrapping, and a row that genuinely overflows is handled by the
			// adaptive layout choosing a narrower arrangement.
			Children: children,
		}

	case app.ModalNode:
		return buildModal(n, path, dispatch)

	case app.OverlayNode:
		overlay := &render.Overlay{CompID: path}
		if n.Base != nil {
			overlay.Base = build(n.Base, childPath(path, 0), dispatch)
		}
		for i, layer := range n.Layers {
			if layer.Content == nil {
				continue
			}
			overlay.Layers = append(overlay.Layers, render.OverlayLayer{
				Anchor:  overlayAnchor(layer.Anchor),
				Inset:   layer.Inset,
				Content: build(layer.Content, childPath(path, i+1), dispatch),
			})
		}
		return overlay

	default:
		panic(fmt.Sprintf("poem: unsupported node type %T", node))
	}
}

// overlayAnchor maps the app IR anchor onto the render layer's. Both enums list
// the nine positions in the same order, so this is total.
func overlayAnchor(a app.OverlayAnchor) render.OverlayAnchor {
	switch a {
	case app.OverlayTop:
		return render.AnchorTop
	case app.OverlayTopRight:
		return render.AnchorTopRight
	case app.OverlayLeft:
		return render.AnchorLeft
	case app.OverlayCenter:
		return render.AnchorCenter
	case app.OverlayRight:
		return render.AnchorRight
	case app.OverlayBottomLeft:
		return render.AnchorBottomLeft
	case app.OverlayBottom:
		return render.AnchorBottom
	case app.OverlayBottomRight:
		return render.AnchorBottomRight
	default:
		return render.AnchorTopLeft
	}
}

func desktopActionLabel(icon app.IconID, label string) string {
	glyph := ""
	switch icon {
	case app.IconSearch:
		glyph = "⌕"
	case app.IconRoute, app.IconDirections:
		glyph = "↝"
	case app.IconRefresh:
		glyph = "↻"
	case app.IconAdd:
		glyph = "+"
	case app.IconRemove:
		glyph = "−"
	case app.IconSettings:
		glyph = "⚙"
	case app.IconMore:
		glyph = "⋯"
	case app.IconClose, app.IconClear:
		glyph = "×"
	case app.IconMap:
		glyph = "▧"
	case app.IconLocation:
		glyph = "●"
	case app.IconSwap:
		glyph = "⇅"
	case app.IconConnection:
		glyph = "◉"
	}
	if glyph == "" {
		return label
	}
	return glyph + "  " + label
}

// buildModal returns the real trigger button for a ModalNode. The modal
// itself is never part of the returned tree — POEM's Modal lives in the
// OverlayManager, not Pages[...] — so opening it is a one-shot side effect
// inside the trigger's OnClick, matching how real POEM apps (cmd/gallery)
// already open a Modal: from directly inside a click handler, not from
// BuildPagesFn's own per-repaint execution. That's deliberate, not
// incidental — calling OpenModal again on every repaint while already open
// would replace the whole OverlayEntry (see okf/architecture/overview.md)
// and steal focus from wherever the user just tabbed to inside it. Content
// is therefore a snapshot from the moment the trigger was clicked, not
// continuously live against later unrelated state changes.
func buildModal(n app.ModalNode, path string, dispatch func(app.Msg)) render.Component {
	content := make([]render.Component, len(n.Content))
	for i, child := range n.Content {
		content[i] = build(child, childPath(path, i), dispatch)
	}
	titleLabel := render.NewLabel(path+"/title", n.Title)
	body := &render.FlexBox{
		CompID:    path + "/body",
		Direction: render.Vertical,
		Gap:       8,
		Padding:   16,
		Children:  append([]render.Component{titleLabel}, content...),
	}

	modalID := path + "_modal"
	return render.NewButton(path, n.Trigger, func(rstate *render.ApplicationState) {
		w, h := rstate.GetWindowSize()
		const cardWidth, cardHeight = 360, 280
		cardRect := image.Rect((w-cardWidth)/2, (h-cardHeight)/2, (w+cardWidth)/2, (h+cardHeight)/2)
		body.SetBounds(cardRect)
		modal := &render.Modal{
			CompID:            modalID,
			Rect:              image.Rect(0, 0, w, h),
			CardRect:          cardRect,
			Children:          []render.Component{body},
			DismissOnBackdrop: true,
		}
		rstate.OpenModal(modalID, modal, nil)
	})
}

// poemVariant maps a app.Variant onto POEM's theme variant. app.Variant's
// members are a subset of POEM's, so this is total with no fallback needed
// beyond the neutral default.
func poemVariant(v app.Variant) render.ThemeVariant {
	switch v {
	case app.VariantPrimary:
		return render.VariantPrimary
	case app.VariantSecondary:
		return render.VariantSecondary
	case app.VariantDanger:
		return render.VariantDanger
	case app.VariantSuccess:
		return render.VariantSuccess
	case app.VariantWarning:
		return render.VariantWarning
	default:
		return render.VariantNeutral
	}
}

func imageMarkerVariant(v app.Variant) render.ImageMarkerVariant {
	switch v {
	case app.VariantPrimary:
		return render.ImageMarkerPrimary
	case app.VariantSecondary:
		return render.ImageMarkerSecondary
	case app.VariantDanger:
		return render.ImageMarkerDanger
	case app.VariantSuccess:
		return render.ImageMarkerSuccess
	case app.VariantWarning:
		return render.ImageMarkerWarning
	default:
		return render.ImageMarkerNeutral
	}
}

func flexDirection(d app.Direction) render.LayoutDirection {
	if d == app.Horizontal {
		return render.Horizontal
	}
	return render.Vertical
}

func childPath(parent string, index int) string {
	return fmt.Sprintf("%s/%d", parent, index)
}

func semanticPath(parent, id string) string {
	if id == "" {
		return parent
	}
	return parent + "/id-" + id
}
