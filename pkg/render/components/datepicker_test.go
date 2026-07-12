package components

import (
	"image"
	"testing"

	"github.com/mulavdm/poem/pkg/render/semantics"
	renderstate "github.com/mulavdm/poem/pkg/render/state"
	"github.com/mulavdm/poem/pkg/render/types"
)

func datePickerTestState() *types.ApplicationState {
	return &types.ApplicationState{Overlays: types.NewOverlayManager(), TransientState: renderstate.NewStore()}
}

func TestDateValidationAndParsing(t *testing.T) {
	if !NewDate(2028, 2, 29).Valid() || NewDate(2027, 2, 29).Valid() {
		t.Fatal("leap-day validation failed")
	}
	parsed, err := ParseDate("2026-06-21")
	if err != nil || parsed != NewDate(2026, 6, 21) || parsed.String() != "2026-06-21" {
		t.Fatalf("parsed=%+v err=%v", parsed, err)
	}
}

func TestDatePickerSemanticSelectionHonorsRange(t *testing.T) {
	state := datePickerTestState()
	selected := Date{}
	picker := NewDatePicker("due", NewDate(2026, 6, 15), func(value Date, _ *types.ApplicationState) { selected = value })
	picker.Min, picker.Max = NewDate(2026, 6, 10), NewDate(2026, 6, 20)
	picker.Rect = image.Rect(0, 0, 240, 36)
	if !picker.open(state) {
		t.Fatal("calendar did not open")
	}
	popup := state.Overlays.Snapshot()[0].Component.(*calendarPopup)
	if popup.PerformSemanticAction("due.calendar/date/2026-06-25", "select", "", state) {
		t.Fatal("out-of-range date was selected")
	}
	if !popup.PerformSemanticAction("due.calendar/date/2026-06-18", "select", "", state) || selected != NewDate(2026, 6, 18) {
		t.Fatalf("selected=%+v", selected)
	}
	if len(state.Overlays.Snapshot()) != 0 {
		t.Fatal("calendar remained open after selection")
	}
}

func TestDatePickerKeyboardStateSurvivesRebuild(t *testing.T) {
	state := datePickerTestState()
	state.FocusedID = "due"
	value := NewDate(2026, 6, 15)
	first := NewDatePicker("due", value, nil)
	first.Rect = image.Rect(0, 0, 240, 36)
	if !first.OnKey(13, 0, state) || !first.OnKey(0x27, 0, state) {
		t.Fatal("calendar keyboard interaction failed")
	}
	selected := Date{}
	rebuilt := NewDatePicker("due", value, func(next Date, _ *types.ApplicationState) { selected = next })
	rebuilt.Rect = first.Rect
	if !rebuilt.OnKey(13, 0, state) || selected != NewDate(2026, 6, 16) {
		t.Fatalf("retained keyboard date selected %+v", selected)
	}
}

func TestDatePickerSemanticSetValue(t *testing.T) {
	state := datePickerTestState()
	picker := NewDatePicker("due", Date{}, nil)
	if !picker.PerformSemanticAction("due", "set-value", "2026-12-24", state) || picker.Value != NewDate(2026, 12, 24) {
		t.Fatalf("value=%+v", picker.Value)
	}
	if picker.PerformSemanticAction("due", "set-value", "not-a-date", state) {
		t.Fatal("invalid semantic date was accepted")
	}
}

func TestDatePickerCalendarKeyboardNavigation(t *testing.T) {
	state := datePickerTestState()
	state.FocusedID = "due"
	selected := Date{}
	picker := NewDatePicker("due", NewDate(2027, 1, 31), func(value Date, _ *types.ApplicationState) { selected = value })
	picker.Rect = image.Rect(0, 0, 240, 36)
	if !picker.OnKey(13, 0, state) || !picker.OnKey(0x22, 0, state) { // open, Page Down
		t.Fatal("calendar Page Down was not handled")
	}
	interaction := picker.interaction(state)
	if interaction.Highlighted != NewDate(2027, 2, 28) {
		t.Fatalf("month-clamped highlight=%+v", interaction.Highlighted)
	}
	if !picker.OnKey(0x24, 0, state) || picker.interaction(state).Highlighted != NewDate(2027, 2, 28) {
		t.Fatalf("range-independent Home changed Sunday %+v", picker.interaction(state).Highlighted)
	}
	if !picker.OnKey(0x23, 0, state) || picker.interaction(state).Highlighted != NewDate(2027, 3, 6) {
		t.Fatalf("End highlight=%+v", picker.interaction(state).Highlighted)
	}
	if !picker.OnKey(13, 0, state) || selected != NewDate(2027, 3, 6) {
		t.Fatalf("keyboard selected %+v", selected)
	}
}

func TestDatePickerKeyboardClampsToAllowedRange(t *testing.T) {
	state := datePickerTestState()
	state.FocusedID = "due"
	picker := NewDatePicker("due", NewDate(2026, 6, 10), nil)
	picker.Min, picker.Max = NewDate(2026, 6, 10), NewDate(2026, 6, 20)
	picker.Rect = image.Rect(0, 0, 240, 36)
	if !picker.OnKey(13, 0, state) || !picker.OnKey(0x24, 0, state) || picker.interaction(state).Highlighted != picker.Min {
		t.Fatalf("Home did not clamp to minimum: %+v", picker.interaction(state).Highlighted)
	}
	if !picker.OnKey(0x23, 0, state) || picker.interaction(state).Highlighted != NewDate(2026, 6, 13) {
		t.Fatalf("End after minimum=%+v", picker.interaction(state).Highlighted)
	}
	if !picker.OnKey(0x22, 0, state) || picker.interaction(state).Highlighted != picker.Max {
		t.Fatalf("Page Down did not clamp to maximum: %+v", picker.interaction(state).Highlighted)
	}
}

func TestCalendarPublishesGridCoordinatesAndActiveFocus(t *testing.T) {
	state := datePickerTestState()
	state.FocusedID = "due"
	picker := NewDatePicker("due", NewDate(2026, 6, 21), nil)
	picker.Rect = image.Rect(0, 0, 240, 36)
	if !picker.open(state) {
		t.Fatal("calendar did not open")
	}
	popup := state.Overlays.Snapshot()[0].Component.(*calendarPopup)
	node := popup.Semantics(state)
	if node.Grid == nil || node.Grid.Rows != 6 || node.Grid.Columns != 7 || node.Collection == nil || len(node.Children) != 51 {
		t.Fatalf("calendar grid=%+v collection=%+v children=%d", node.Grid, node.Collection, len(node.Children))
	}
	if node.Children[2].Role != semantics.RoleColumnHeader || node.Children[2].Name != "Sunday" {
		t.Fatalf("first weekday header=%+v", node.Children[2])
	}
	var active semantics.Node
	for _, child := range node.Children {
		if child.ID == "due.calendar/date/2026-06-21" {
			active = child
			break
		}
	}
	if active.GridItem == nil || active.GridItem.Row != 3 || active.GridItem.Column != 0 || !active.State.Selected || !active.State.Focused {
		t.Fatalf("active date semantics=%+v", active)
	}
	if picker.Semantics(state).State.Focused {
		t.Fatal("owner and active grid cell both reported keyboard focus")
	}
	if popup.PerformSemanticAction(active.ID, semantics.ActionInvoke, "", state) {
		t.Fatal("date cell accepted unadvertised invoke action")
	}
}

func TestCalendarMetricsScaleFromThemeTokens(t *testing.T) {
	state := datePickerTestState()
	state.TextScale = 1.5
	picker := NewDatePicker("due", NewDate(2026, 6, 21), nil)
	picker.Rect = image.Rect(0, 0, 180, 40)
	if !picker.open(state) {
		t.Fatal("calendar did not open")
	}
	popup := state.Overlays.Snapshot()[0].Component.(*calendarPopup)
	theme := activeTheme(state)
	if popup.HeaderHeight != theme.Controls.Medium || popup.WeekdayHeight != theme.Controls.Small || popup.CellHeight != theme.Controls.Medium || popup.Rect.Dx() < 7*theme.Controls.Medium {
		t.Fatalf("calendar metrics header=%d weekday=%d cell=%d bounds=%v theme=%+v", popup.HeaderHeight, popup.WeekdayHeight, popup.CellHeight, popup.Rect, theme.Controls)
	}
}
