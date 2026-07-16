// Package poem drives a Trellis app.App as a real running POEM desktop
// application.
package desktop

import (
	"fmt"
	"image"

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
	case app.TextNode:
		return render.NewLabel(path, n.Value)

	case app.ButtonNode:
		button := render.NewButton(path, n.Label, func(*render.ApplicationState) {
			dispatch(n.OnClick)
		})
		button.Disabled = n.Disabled
		return button

	case app.TextInputNode:
		input := render.NewTextInput(path, n.Placeholder)
		input.Value = n.Value
		msg := n.OnChange
		input.OnChange = func(value string, _ *render.ApplicationState) {
			dispatch(app.Msg{Name: msg.Name, Payload: value})
		}
		return input

	case app.TextAreaNode:
		area := render.NewTextArea(path, n.Placeholder)
		area.Value = n.Value
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
			Children:  children,
		}

	case app.ModalNode:
		return buildModal(n, path, dispatch)

	default:
		panic(fmt.Sprintf("poem: unsupported node type %T", node))
	}
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

func flexDirection(d app.Direction) render.LayoutDirection {
	if d == app.Horizontal {
		return render.Horizontal
	}
	return render.Vertical
}

func childPath(parent string, index int) string {
	return fmt.Sprintf("%s/%d", parent, index)
}
