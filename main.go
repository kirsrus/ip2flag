package main

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/getlantern/systray"
	"github.com/iamcalledrob/netstatus"
	"github.com/kirsrus/ip2flag/internal/png2ico"
)

//go:embed flags
var flagsFS embed.FS

// Версия программы из Git. Передаётся при компиляции программы.
var version string = "v0.0.0"

// updateIcon устанавливает иконку в трее в зависимости от наличия интернета:
// интернет отсутствует — c_off.png, интернет есть — c_unknown.png.
func updateIcon(available bool, offIcoData, unknownIcoData []byte) {
	if available {
		systray.SetIcon(unknownIcoData)
	} else {
		systray.SetIcon(offIcoData)
	}
}

func main() {
	// Читаем PNG-иконки состояния интернета из встроенной в бинарник файловой системы.
	offIconData, err := flagsFS.ReadFile("flags/c_off.png")
	if err != nil {
		log.Fatalf("не удалось прочитать иконку flags/c_off.png: %v", err)
	}
	unknownIconData, err := flagsFS.ReadFile("flags/c_unknown.png")
	if err != nil {
		log.Fatalf("не удалось прочитать иконку flags/c_unknown.png: %v", err)
	}

	// В системном трее Windows отображаются иконки в формате ICO,
	// поэтому конвертируем PNG в ICO на лету.
	offIcoData, err := png2ico.Png2ICO(offIconData)
	if err != nil {
		log.Fatalf("не удалось преобразовать иконку c_off.png в ICO: %v", err)
	}
	unknownIcoData, err := png2ico.Png2ICO(unknownIconData)
	if err != nil {
		log.Fatalf("не удалось преобразовать иконку c_unknown.png в ICO: %v", err)
	}

	// Run блокирует выполнение до вызова systray.Quit().
	systray.Run(
		func() {
			systray.SetTooltip(fmt.Sprintf("ip2flag %s", version))

			// Запускаем мониторинг состояния интернета.
			monitor := netstatus.StartMonitor(context.Background())

			// Устанавливаем иконку в соответствии с текущим состоянием интернета.
			updateIcon(monitor.Current(context.Background()).Available, offIcoData, unknownIcoData)

			// Обновляем иконку при каждом изменении состояния интернета.
			// Внутри колбэка нельзя вызывать monitor.Current (приведёт к deadlock),
			// поэтому используем статус, переданный в аргументе.
			monitor.OnChange(func(s netstatus.Status) {
				updateIcon(s.Available, offIcoData, unknownIcoData)
			})

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
