// Package web wbudowuje zbudowany frontend (web/dist) w binarkę Go.
//
// Katalog dist zawiera zawsze .gitkeep, żeby kompilacja działała także przed
// pierwszym `npm run build`. Wtedy serwer pokazuje stronę z instrukcją.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist zwraca zawartość katalogu dist jako system plików z index.html w korzeniu.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// fs.Sub zawodzi tylko przy niepoprawnej ścieżce, a ta jest stała.
		panic(err)
	}
	return sub
}
