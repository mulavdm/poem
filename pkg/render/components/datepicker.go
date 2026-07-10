package components

import (
	"fmt"
	"image"
	"time"

	"go_native_gpu_gui/pkg/render/semantics"
	renderstate "go_native_gpu_gui/pkg/render/state"
	"go_native_gpu_gui/pkg/render/types"
)

// Date is a timezone-free civil date suitable for application values.
type Date struct {
	Year  int
	Month int
	Day   int
}

func NewDate(year, month, day int) Date { return Date{Year: year, Month: month, Day: day} }
func (d Date) Valid() bool {
	if d.Year < 1 || d.Month < 1 || d.Month > 12 || d.Day < 1 {
		return false
	}
	t := time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
	return t.Year() == d.Year && int(t.Month()) == d.Month && t.Day() == d.Day
}
func (d Date) String() string {
	if !d.Valid() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}
func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return Date{}, err
	}
	return Date{Year: parsed.Year(), Month: int(parsed.Month()), Day: parsed.Day()}, nil
}
func (d Date) time() time.Time {
	return time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
}
func (d Date) AddDays(days int) Date {
	if !d.Valid() {
		return d
	}
	next := d.time().AddDate(0, 0, days)
	return Date{Year: next.Year(), Month: int(next.Month()), Day: next.Day()}
}
func compareDate(a, b Date) int {
	aValue := a.Year*10000 + a.Month*100 + a.Day
	bValue := b.Year*10000 + b.Month*100 + b.Day
	if aValue < bValue {
		return -1
	}
	if aValue > bValue {
		return 1
	}
	return 0
}

type datePickerInteraction struct {
	DisplayYear  int
	DisplayMonth int
	Highlighted  Date
}

type DatePicker struct {
	CompID         string
	Rect           image.Rectangle
	Value          Date
	Min            Date
	Max            Date
	Placeholder    string
	AccessibleName string
	Disabled       bool
	ReadOnly       bool
	Invalid        bool
	OnChange       func(Date, *types.ApplicationState)
}

func NewDatePicker(id string, value Date, onChange func(Date, *types.ApplicationState)) *DatePicker {
	return &DatePicker{CompID: id, Value: value, Placeholder: "Select date", OnChange: onChange}
}
func (d *DatePicker) ID() string                    { return d.CompID }
func (d *DatePicker) GetID() string                 { return d.CompID }
func (d *DatePicker) Bounds() image.Rectangle       { return d.Rect }
func (d *DatePicker) SetBounds(r image.Rectangle)   { d.Rect = r }
func (d *DatePicker) Focusable() bool               { return !d.Disabled }
func (d *DatePicker) Walk(fn func(types.Component)) { fn(d) }
func (d *DatePicker) overlayID() string             { return d.CompID + ".calendar" }
func (d *DatePicker) interactionKey() string        { return d.CompID + "/date-picker" }
func (d *DatePicker) Measure(avail image.Point, state *types.ApplicationState) types.MeasureResult {
	th := activeTheme(state)
	width := avail.X
	if width <= 0 {
		width = 220
	}
	return types.MeasureResult{Preferred: applyExplicitSize(explicitSize(d.Rect), image.Pt(width, th.Controls.Medium)), Min: image.Pt(160, th.Controls.Medium)}
}
func (d *DatePicker) valueLabel() string {
	if d.Value.Valid() {
		return d.Value.String()
	}
	return d.Placeholder
}
func (d *DatePicker) Draw(p types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	visual := buttonVisual(th, 0, state != nil && state.HoveredID == d.CompID, state != nil && state.ActiveID == d.CompID, d.Disabled, false, nil)
	if d.Invalid {
		visual.border = th.Colors.Danger
	}
	drawControlSurface(p, d.Rect, visual, state != nil && state.FocusedID == d.CompID && !d.Disabled)
	col := visual.foreground
	if !d.Value.Valid() {
		col = th.Colors.TextMuted
	}
	p.DrawText(d.valueLabel(), d.Rect.Min.X+th.Spacing.MD, d.Rect.Min.Y+d.Rect.Dy()/2+5, col)
	icon := image.Rect(d.Rect.Max.X-th.Spacing.LG-10, d.Rect.Min.Y+d.Rect.Dy()/2-7, d.Rect.Max.X-th.Spacing.LG+2, d.Rect.Min.Y+d.Rect.Dy()/2+7)
	p.DrawLine(icon.Min.X, icon.Min.Y+3, icon.Max.X, icon.Min.Y+3, visual.foreground)
	p.DrawLine(icon.Min.X, icon.Min.Y, icon.Min.X, icon.Max.Y, visual.foreground)
	p.DrawLine(icon.Max.X, icon.Min.Y, icon.Max.X, icon.Max.Y, visual.foreground)
	p.DrawLine(icon.Min.X, icon.Max.Y, icon.Max.X, icon.Max.Y, visual.foreground)
}
func (d *DatePicker) HitTest(pt image.Point) string {
	if pt.In(d.Rect) {
		return d.CompID
	}
	return ""
}
func (d *DatePicker) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	if d.Disabled || d.ReadOnly || !pt.In(d.Rect) {
		return false
	}
	state.FocusedID = d.CompID
	state.ActiveID = d.CompID
	return true
}
func (d *DatePicker) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	if state.ActiveID != d.CompID || !pt.In(d.Rect) {
		return false
	}
	if d.isOpen(state) {
		d.close(state)
	} else {
		d.open(state)
	}
	return true
}
func (d *DatePicker) OnMouseMove(image.Point, *types.ApplicationState) bool { return false }
func (d *DatePicker) allowed(value Date) bool {
	if !value.Valid() {
		return false
	}
	if d.Min.Valid() && compareDate(value, d.Min) < 0 {
		return false
	}
	if d.Max.Valid() && compareDate(value, d.Max) > 0 {
		return false
	}
	return true
}
func (d *DatePicker) initialInteraction() datePickerInteraction {
	base := d.Value
	if !base.Valid() {
		now := time.Now()
		base = Date{Year: now.Year(), Month: int(now.Month()), Day: now.Day()}
	}
	base = d.clamp(base)
	return datePickerInteraction{DisplayYear: base.Year, DisplayMonth: base.Month, Highlighted: base}
}
func (d *DatePicker) clamp(value Date) Date {
	if d.Min.Valid() && compareDate(value, d.Min) < 0 {
		return d.Min
	}
	if d.Max.Valid() && compareDate(value, d.Max) > 0 {
		return d.Max
	}
	return value
}
func addMonthsClamped(value Date, months int) Date {
	if !value.Valid() {
		return value
	}
	target := time.Date(value.Year, time.Month(value.Month)+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	lastDay := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := value.Day
	if day > lastDay {
		day = lastDay
	}
	return Date{Year: target.Year(), Month: int(target.Month()), Day: day}
}
func (d *DatePicker) interaction(state *types.ApplicationState) datePickerInteraction {
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	if interaction, ok := renderstate.Load[datePickerInteraction](state.TransientState, d.interactionKey()); ok {
		return interaction
	}
	interaction := d.initialInteraction()
	renderstate.StoreValue(state.TransientState, d.interactionKey(), interaction)
	return interaction
}
func (d *DatePicker) storeInteraction(state *types.ApplicationState, interaction datePickerInteraction) {
	if state.TransientState == nil {
		state.TransientState = renderstate.NewStore()
	}
	renderstate.StoreValue(state.TransientState, d.interactionKey(), interaction)
}
func (d *DatePicker) isOpen(state *types.ApplicationState) bool {
	if state == nil || state.Overlays == nil {
		return false
	}
	for _, overlay := range state.Overlays.Snapshot() {
		if overlay.ID == d.overlayID() {
			return true
		}
	}
	return false
}
func (d *DatePicker) open(state *types.ApplicationState) bool {
	if d.Disabled || d.ReadOnly {
		return false
	}
	interaction := d.interaction(state)
	th := activeTheme(state)
	headerHeight := th.Controls.Medium
	weekdayHeight := th.Controls.Small
	cellHeight := th.Controls.Medium
	minWidth := 7*cellHeight + 2*th.Spacing.XS
	maxWidth := 7*th.Controls.Large + 2*th.Spacing.XS
	width := d.Rect.Dx()
	if width < minWidth {
		width = minWidth
	} else if width > maxWidth {
		width = maxWidth
	}
	height := headerHeight + weekdayHeight + 6*cellHeight + 2*th.Spacing.XS
	top := d.Rect.Max.Y + th.Spacing.XS
	popup := &calendarPopup{CompID: d.overlayID(), Rect: image.Rect(d.Rect.Min.X, top, d.Rect.Min.X+width, top+height), Owner: d, Interaction: interaction, HeaderHeight: headerHeight, WeekdayHeight: weekdayHeight, CellHeight: cellHeight}
	state.OpenAnchoredOverlay(d.overlayID(), popup, true)
	return true
}
func (d *DatePicker) close(state *types.ApplicationState) {
	state.CloseOverlay(d.overlayID())
	if state.TransientState != nil {
		state.TransientState.Delete(d.interactionKey())
	}
}
func (d *DatePicker) publish(value Date, state *types.ApplicationState) bool {
	if !d.allowed(value) {
		return false
	}
	if d.OnChange != nil {
		d.OnChange(value, state)
	} else {
		d.Value = value
	}
	d.close(state)
	return true
}
func (d *DatePicker) OnKey(key uint32, _ rune, state *types.ApplicationState) bool {
	if state.FocusedID != d.CompID || d.Disabled || d.ReadOnly {
		return false
	}
	if !d.isOpen(state) {
		if key == 13 || key == 32 || key == 0x28 {
			return d.open(state)
		}
		return false
	}
	interaction := d.interaction(state)
	switch key {
	case 0x1B:
		d.close(state)
		return true
	case 0x25:
		interaction.Highlighted = interaction.Highlighted.AddDays(-1)
	case 0x27:
		interaction.Highlighted = interaction.Highlighted.AddDays(1)
	case 0x26:
		interaction.Highlighted = interaction.Highlighted.AddDays(-7)
	case 0x28:
		interaction.Highlighted = interaction.Highlighted.AddDays(7)
	case 0x24: // Home: first day of the current week
		interaction.Highlighted = interaction.Highlighted.AddDays(-int(interaction.Highlighted.time().Weekday()))
	case 0x23: // End: last day of the current week
		interaction.Highlighted = interaction.Highlighted.AddDays(6 - int(interaction.Highlighted.time().Weekday()))
	case 0x21: // Page Up: previous month
		interaction.Highlighted = addMonthsClamped(interaction.Highlighted, -1)
	case 0x22: // Page Down: next month
		interaction.Highlighted = addMonthsClamped(interaction.Highlighted, 1)
	case 13, 32:
		return d.publish(interaction.Highlighted, state)
	default:
		return false
	}
	interaction.Highlighted = d.clamp(interaction.Highlighted)
	interaction.DisplayYear, interaction.DisplayMonth = interaction.Highlighted.Year, interaction.Highlighted.Month
	d.storeInteraction(state, interaction)
	return d.open(state)
}
func (d *DatePicker) Semantics(state *types.ApplicationState) semantics.Node {
	name := d.AccessibleName
	if name == "" {
		name = d.Placeholder
	}
	open := d.isOpen(state)
	return semantics.Node{ID: d.CompID, Role: semantics.RoleDatePicker, Name: name, Value: d.Value.String(), Bounds: d.Rect, State: semantics.State{Disabled: d.Disabled, ReadOnly: d.ReadOnly, Invalid: d.Invalid, Focused: state != nil && state.FocusedID == d.CompID && !open, Expanded: open}, Actions: []semantics.Action{semantics.ActionFocus, semantics.ActionSetValue, semantics.ActionExpand, semantics.ActionCollapse}}
}
func (d *DatePicker) PerformSemanticAction(targetID string, action semantics.Action, value string, state *types.ApplicationState) bool {
	if targetID != d.CompID || d.Disabled {
		return false
	}
	switch action {
	case semantics.ActionFocus:
		state.FocusedID = d.CompID
		return true
	case semantics.ActionExpand:
		return d.open(state)
	case semantics.ActionCollapse:
		d.close(state)
		return true
	case semantics.ActionSetValue:
		if d.ReadOnly {
			return false
		}
		parsed, err := ParseDate(value)
		return err == nil && d.publish(parsed, state)
	}
	return false
}

type calendarPopup struct {
	CompID                                  string
	Rect                                    image.Rectangle
	Owner                                   *DatePicker
	Interaction                             datePickerInteraction
	HeaderHeight, WeekdayHeight, CellHeight int
}

func (c *calendarPopup) ID() string                    { return c.CompID }
func (c *calendarPopup) GetID() string                 { return c.CompID }
func (c *calendarPopup) Bounds() image.Rectangle       { return c.Rect }
func (c *calendarPopup) SetBounds(r image.Rectangle)   { c.Rect = r }
func (c *calendarPopup) Focusable() bool               { return false }
func (c *calendarPopup) Walk(fn func(types.Component)) { fn(c) }
func (c *calendarPopup) headerRect() image.Rectangle {
	return image.Rect(c.Rect.Min.X, c.Rect.Min.Y, c.Rect.Max.X, c.Rect.Min.Y+c.HeaderHeight)
}
func (c *calendarPopup) weekdayRect(column int) image.Rectangle {
	return image.Rect(c.Rect.Min.X+column*c.Rect.Dx()/7, c.headerRect().Max.Y, c.Rect.Min.X+(column+1)*c.Rect.Dx()/7, c.headerRect().Max.Y+c.WeekdayHeight)
}
func (c *calendarPopup) gridTop() int { return c.Rect.Min.Y + c.HeaderHeight + c.WeekdayHeight }
func (c *calendarPopup) cellRect(index int) image.Rectangle {
	row, column := index/7, index%7
	return image.Rect(c.Rect.Min.X+column*c.Rect.Dx()/7, c.gridTop()+row*c.CellHeight, c.Rect.Min.X+(column+1)*c.Rect.Dx()/7, c.gridTop()+(row+1)*c.CellHeight)
}
func (c *calendarPopup) firstVisible() Date {
	first := time.Date(c.Interaction.DisplayYear, time.Month(c.Interaction.DisplayMonth), 1, 0, 0, 0, 0, time.UTC)
	start := first.AddDate(0, 0, -int(first.Weekday()))
	return Date{Year: start.Year(), Month: int(start.Month()), Day: start.Day()}
}
func (c *calendarPopup) dateAt(index int) Date { return c.firstVisible().AddDays(index) }
func (c *calendarPopup) Draw(p types.Painter, state *types.ApplicationState) {
	th := activeTheme(state)
	p.SetShadow(float32(th.Elevation.High.OffsetX), float32(th.Elevation.High.OffsetY), float32(th.Elevation.High.Blur))
	p.DrawRoundedRect(c.Rect, th.Radii.Medium, th.Colors.SurfaceRaised)
	p.SetShadow(0, 0, 0)
	header := c.headerRect()
	monthLabel := time.Month(c.Interaction.DisplayMonth).String() + " " + intString(c.Interaction.DisplayYear)
	p.DrawText("<", header.Min.X+th.Spacing.MD, header.Min.Y+header.Dy()/2+5, th.Colors.Text)
	p.DrawText(monthLabel, header.Min.X+header.Dx()/2-len([]rune(monthLabel))*fontCharWidth(state)/2, header.Min.Y+header.Dy()/2+5, th.Colors.Text)
	p.DrawText(">", header.Max.X-th.Spacing.LG, header.Min.Y+header.Dy()/2+5, th.Colors.Text)
	weekdays := [...]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	for i, weekday := range weekdays {
		r := c.weekdayRect(i)
		p.DrawText(weekday, r.Min.X+(r.Dx()-2*fontCharWidth(state))/2, r.Min.Y+r.Dy()/2+5, th.Colors.TextMuted)
	}
	for index := 0; index < 42; index++ {
		date := c.dateAt(index)
		r := c.cellRect(index)
		selected := c.Owner.Value.Valid() && compareDate(date, c.Owner.Value) == 0
		highlighted := compareDate(date, c.Interaction.Highlighted) == 0
		disabled := !c.Owner.allowed(date)
		id := c.CompID + "/date/" + date.String()
		visual := buttonVisual(th, 0, state.HoveredID == id, state.ActiveID == id, disabled, selected || highlighted, nil)
		visual.border = colorZero()
		if selected || highlighted || state.HoveredID == id {
			drawControlSurface(p, r, visual, highlighted)
		}
		col := visual.foreground
		if date.Month != c.Interaction.DisplayMonth && !disabled {
			col = th.Colors.TextMuted
		}
		label := intString(date.Day)
		p.DrawText(label, r.Min.X+(r.Dx()-len(label)*fontCharWidth(state))/2, r.Min.Y+r.Dy()/2+5, col)
	}
}
func (c *calendarPopup) HitTest(pt image.Point) string {
	if !pt.In(c.Rect) {
		return ""
	}
	header := c.headerRect()
	if pt.In(image.Rect(header.Min.X, header.Min.Y, header.Min.X+c.HeaderHeight, header.Max.Y)) {
		return c.CompID + "/previous-month"
	}
	if pt.In(image.Rect(header.Max.X-c.HeaderHeight, header.Min.Y, header.Max.X, header.Max.Y)) {
		return c.CompID + "/next-month"
	}
	if pt.Y >= c.gridTop() {
		column := (pt.X - c.Rect.Min.X) * 7 / c.Rect.Dx()
		row := (pt.Y - c.gridTop()) / c.CellHeight
		index := row*7 + column
		if index >= 0 && index < 42 {
			return c.CompID + "/date/" + c.dateAt(index).String()
		}
	}
	return c.CompID
}
func (c *calendarPopup) OnMouseDown(pt image.Point, state *types.ApplicationState) bool {
	id := c.HitTest(pt)
	if date, ok := c.targetDate(id); ok && !c.Owner.allowed(date) {
		return false
	}
	state.ActiveID = id
	return id != "" && id != c.CompID
}
func (c *calendarPopup) OnMouseUp(pt image.Point, state *types.ApplicationState) bool {
	id := c.HitTest(pt)
	if id == "" || id != state.ActiveID {
		return false
	}
	return c.PerformSemanticAction(id, semantics.ActionInvoke, "", state)
}
func (c *calendarPopup) OnKey(uint32, rune, *types.ApplicationState) bool { return false }
func (c *calendarPopup) OnMouseMove(pt image.Point, state *types.ApplicationState) bool {
	date, ok := c.targetDate(c.HitTest(pt))
	if !ok || !c.Owner.allowed(date) || compareDate(date, c.Interaction.Highlighted) == 0 {
		return false
	}
	c.Interaction.Highlighted = date
	c.Interaction.DisplayYear, c.Interaction.DisplayMonth = date.Year, date.Month
	c.Owner.storeInteraction(state, c.Interaction)
	return true
}
func (c *calendarPopup) targetDate(targetID string) (Date, bool) {
	prefix := c.CompID + "/date/"
	if len(targetID) <= len(prefix) || targetID[:len(prefix)] != prefix {
		return Date{}, false
	}
	date, err := ParseDate(targetID[len(prefix):])
	return date, err == nil
}
func (c *calendarPopup) navigate(months int, state *types.ApplicationState) bool {
	c.Interaction.Highlighted = c.Owner.clamp(addMonthsClamped(c.Interaction.Highlighted, months))
	c.Interaction.DisplayYear, c.Interaction.DisplayMonth = c.Interaction.Highlighted.Year, c.Interaction.Highlighted.Month
	c.Owner.storeInteraction(state, c.Interaction)
	return true
}
func (c *calendarPopup) Semantics(state *types.ApplicationState) semantics.Node {
	label := time.Month(c.Interaction.DisplayMonth).String() + " " + intString(c.Interaction.DisplayYear)
	node := semantics.Node{ID: c.CompID, Role: semantics.RoleGrid, Name: label, Bounds: c.Rect,
		Collection: &semantics.CollectionValue{Selectable: true, SelectionRequired: true}, Grid: &semantics.GridValue{Rows: 6, Columns: 7}}
	node.Children = append(node.Children,
		semantics.Node{ID: c.CompID + "/previous-month", Role: semantics.RoleButton, Name: "Previous month", Bounds: image.Rect(c.headerRect().Min.X, c.headerRect().Min.Y, c.headerRect().Min.X+c.HeaderHeight, c.headerRect().Max.Y), Actions: []semantics.Action{semantics.ActionInvoke}},
		semantics.Node{ID: c.CompID + "/next-month", Role: semantics.RoleButton, Name: "Next month", Bounds: image.Rect(c.headerRect().Max.X-c.HeaderHeight, c.headerRect().Min.Y, c.headerRect().Max.X, c.headerRect().Max.Y), Actions: []semantics.Action{semantics.ActionInvoke}})
	weekdays := [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	for column, weekday := range weekdays {
		node.Children = append(node.Children, semantics.Node{ID: c.CompID + "/weekday/" + intString(column), Role: semantics.RoleColumnHeader, Name: weekday, Bounds: c.weekdayRect(column)})
	}
	for index := 0; index < 42; index++ {
		date := c.dateAt(index)
		disabled := !c.Owner.allowed(date)
		actions := []semantics.Action(nil)
		if !disabled {
			actions = []semantics.Action{semantics.ActionSelect}
		}
		node.Children = append(node.Children, semantics.Node{ID: c.CompID + "/date/" + date.String(), Role: semantics.RoleGridCell, Name: date.String(), Value: date.String(), Bounds: c.cellRect(index), State: semantics.State{Disabled: disabled, Selected: c.Owner.Value.Valid() && compareDate(date, c.Owner.Value) == 0, Focused: state != nil && state.FocusedID == c.Owner.CompID && compareDate(date, c.Interaction.Highlighted) == 0}, Actions: append([]semantics.Action{semantics.ActionFocus}, actions...), GridItem: &semantics.GridItemValue{Row: index / 7, Column: index % 7, RowSpan: 1, ColumnSpan: 1}})
	}
	return node
}
func (c *calendarPopup) PerformSemanticAction(targetID string, action semantics.Action, _ string, state *types.ApplicationState) bool {
	switch targetID {
	case c.CompID + "/previous-month":
		return action == semantics.ActionInvoke && c.navigate(-1, state)
	case c.CompID + "/next-month":
		return action == semantics.ActionInvoke && c.navigate(1, state)
	}
	date, ok := c.targetDate(targetID)
	if !ok || !c.Owner.allowed(date) {
		return false
	}
	switch action {
	case semantics.ActionFocus:
		state.FocusedID = c.Owner.CompID
		c.Interaction.Highlighted = date
		c.Interaction.DisplayYear, c.Interaction.DisplayMonth = date.Year, date.Month
		c.Owner.storeInteraction(state, c.Interaction)
		return true
	case semantics.ActionSelect:
		return c.Owner.publish(date, state)
	}
	return false
}
