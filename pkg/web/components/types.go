package components

import ()

// Variant selects a component's semantic visual treatment.
type Variant string

const (
	// VariantPrimary identifies the primary action or emphasis.
	VariantPrimary Variant = "primary"
	// VariantSecondary identifies a secondary action or emphasis.
	VariantSecondary Variant = "secondary"
	// VariantDanger identifies a destructive action or error state.
	VariantDanger Variant = "danger"
	// VariantSuccess identifies a successful state.
	VariantSuccess Variant = "success"
	// VariantWarning identifies a warning state.
	VariantWarning Variant = "warning"
	// VariantNeutral identifies a neutral state and is the default.
	VariantNeutral Variant = "neutral"
)

// Size selects a component's visual size.
type Size string

const (
	// SizeSmall is a compact component size.
	SizeSmall Size = "small"
	// SizeMedium is the default component size.
	SizeMedium Size = "medium"
	// SizeLarge is a prominent component size.
	SizeLarge Size = "large"
)

// Attrs holds safe, additional HTML attributes for components that support them.
// Event-handler attributes (for example, onclick) and invalid attribute names are ignored.
type Attrs map[string]string
