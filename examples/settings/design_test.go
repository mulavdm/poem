package settings

import (
	"github.com/mulavdm/poem/pkg/app/designlint"
	"testing"
)

func TestDesignLint(t *testing.T) {
	designlint.AssertClean(t, App.View(App.Init), App.Commands(App.Init))
}
