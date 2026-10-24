package main

import (
	"embed"
	"fmt"
	"log"

	"github.com/getlantern/systray"
	"github.com/kirsrus/ip2flag/internal/png2ico"
)

//go:embed flags
var flagsFS embed.FS

// Версия программы из Git. Передаётся при компиляции программы.
var version string = "v0.0.0"

func main() {
	// Читаем PNG-иконку флага из встроенной в бинарник файловой системы.
	iconData, err := flagsFS.ReadFile("flags/ru.png")
	if err != nil {
		log.Fatalf("не удалось прочитать иконку flags/ru.png: %v", err)
	}

	// В системном трее Windows отображаются иконки в формате ICO,
	// поэтому конвертируем PNG в ICO на лету.
	icoData, err := png2ico.Png2ICO(iconData)
	if err != nil {
		log.Fatalf("не удалось преобразовать иконку в ICO: %v", err)
	}

	// Run блокирует выполнение до вызова systray.Quit().
	systray.Run(
		func() {
			systray.SetIcon(icoData)
			systray.SetTooltip(fmt.Sprintf("ip2flag %s", version))

			quitItem := systray.AddMenuItem("Выход", "Завершить работу программы")
			go func() {
				<-quitItem.ClickedCh
				systray.Quit()
			}()
		},
		func() {
			fmt.Println("Программа завершена.")
		},
	)
}
