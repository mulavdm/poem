//go:build windows

package main

import (
	"image"
	"image/color"

	"github.com/mulavdm/poem/pkg/render"
	"github.com/mulavdm/poem/pkg/render/theme"
	"github.com/mulavdm/poem/pkg/render/types"
	poemwindows "github.com/mulavdm/poem/pkg/windows"
)

var settingsOpen bool

func init() {
	config := render.AppConfig{
		Title: "POEM External Overlay Example", Width: 960, Height: 540,
		BuildPagesFn: buildOverlay,
	}
	poemwindows.MustRegister(config, poemwindows.Metadata{
		Identity: "POEM.ExternalOverlay", Title: config.Title, Width: config.Width, Height: config.Height,
	})
}

func buildOverlay(state *types.ApplicationState) {
	w, h := state.GetWindowSize()
	if state.Pages == nil {
		state.Pages = make(map[string][]types.Component)
	}
	state.CurrentPage = "external_overlay"
	card := render.NewPanel("pause_card")
	card.UseTheme = false
	card.BGColor = color.RGBA{18, 25, 39, 230}
	card.Rounding = 16
	card.SetBounds(image.Rect(w/2-230, h/2-150, w/2+230, h/2+150))

	titleText := "MISSION PAUSED"
	bodyText := "Crumb Horizon is holding position."
	if settingsOpen {
		titleText = "SETTINGS"
		bodyText = "Subtitles: ON    Reduced motion: OFF"
	}
	title := render.NewLabel("pause_title", titleText)
	title.UseTheme = false
	title.Color = color.RGBA{245, 247, 251, 255}
	title.Typography = render.TypographyTitle
	title.SetBounds(image.Rect(w/2-185, h/2-112, w/2+185, h/2-70))

	body := render.NewLabel("pause_body", bodyText)
	body.UseTheme = false
	body.Color = color.RGBA{190, 201, 218, 255}
	body.SetBounds(image.Rect(w/2-185, h/2-58, w/2+185, h/2-18))

	resume := render.NewButton("resume", "Resume mission", func(*types.ApplicationState) {})
	resume.Variant = theme.VariantPrimary
	resume.AccessibleName = "Resume mission"
	resume.SetBounds(image.Rect(w/2-185, h/2+18, w/2+185, h/2+66))

	settingsLabel := "Settings"
	if settingsOpen {
		settingsLabel = "Back"
	}
	settings := render.NewButton("settings", settingsLabel, func(*types.ApplicationState) {
		settingsOpen = !settingsOpen
	})
	settings.Variant = theme.VariantSecondary
	settings.AccessibleName = "Open settings"
	settings.SetBounds(image.Rect(w/2-185, h/2+78, w/2+185, h/2+126))

	state.Pages["external_overlay"] = []types.Component{card, title, body, resume, settings}
}

func main() {}
