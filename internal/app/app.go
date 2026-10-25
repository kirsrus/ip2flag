package app

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/getlantern/systray"
	"github.com/iamcalledrob/netstatus"
	"github.com/kirsrus/ip2flag/internal/colorlog"
	"github.com/kirsrus/ip2flag/internal/model"
	"github.com/kirsrus/ip2flag/internal/png2ico"
)

// Версия программы из Git. Передаётся при компиляции программы.
var version string = "v0.0.0"

//go:embed flags
var flagsFS embed.FS

// updateIcon устанавливает иконку в трее в зависимости от наличия интернета:
// интернет отсутствует — c_off.png, интернет есть — c_unknown.png.
func updateIcon(available bool, offIcoData, unknownIcoData []byte) {
	if available {
		systray.SetIcon(unknownIcoData)
	} else {
		systray.SetIcon(offIcoData)
	}
}

// setCountryIcon устанавливает в трее иконку флага страны по коду country,
// полученному от ipinfo.io. Имя файла флага — код страны в нижнем регистре
// с расширением .png из встроенной в бинарник папки flags.
func setCountryIcon(country string) error {
	code := strings.ToLower(strings.TrimSpace(country))
	// Замены для кодов, для которых в папке flags нет файла с таким именем.
	switch code {
	case "gb":
		code = "uk" // Великобритания хранится как uk.png
	}

	path := "flags/" + code + ".png"
	data, err := flagsFS.ReadFile(path)
	if err != nil {
		return fmt.Errorf("нет флага %s: %w", path, err)
	}

	ico, err := png2ico.Png2ICO(data)
	if err != nil {
		return fmt.Errorf("преобразование %s в ICO: %w", path, err)
	}

	systray.SetIcon(ico)
	return nil
}

// onGeoIPReceived вызывается после успешного опроса ipinfo.io.
// Меняет иконку в трее на флаг страны, определённой по IP.
func onGeoIPReceived(geo *model.GeoIP) {
	if err := setCountryIcon(geo.Country); err != nil {
		slog.Warn("не удалось установить иконку флага страны", slog.String("country", geo.Country), slog.String("error", err.Error()))
		return
	}
	slog.Info("получены данные ipinfo.io, установлена иконка флага страны", slog.String("country", geo.Country))
}

func Run() {
	// Инициализация глобального лога программы.
	slog.SetDefault(slog.New(colorlog.New(os.Stderr, slog.LevelDebug)))
	slog.Info("программа запущена", slog.String("version", version))

	// Читаем PNG-иконки состояния интернета из встроенной в бинарник файловой системы.
	offIconData, err := flagsFS.ReadFile("flags/c_off.png")
	if err != nil {
		slog.Error("не удалось прочитать иконку flags/c_off.png", slog.String("error", err.Error()))
		os.Exit(1)
	}
	unknownIconData, err := flagsFS.ReadFile("flags/c_unknown.png")
	if err != nil {
		slog.Error("не удалось прочитать иконку flags/c_unknown.png", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// В системном трее Windows отображаются иконки в формате ICO,
	// поэтому конвертируем PNG в ICO на лету.
	offIcoData, err := png2ico.Png2ICO(offIconData)
	if err != nil {
		slog.Error("не удалось преобразовать иконку c_off.png в ICO", slog.String("error", err.Error()))
		os.Exit(1)
	}
	unknownIcoData, err := png2ico.Png2ICO(unknownIconData)
	if err != nil {
		slog.Error("не удалось преобразовать иконку c_unknown.png в ICO", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Состояние опроса сервиса ipinfo.io (общее для приложения).
	var geo geoState

	// Run блокирует выполнение до вызова systray.Quit().
	systray.Run(
		func() {
			systray.SetTooltip(fmt.Sprintf("ip2flag %s", version))

			// Запускаем мониторинг состояния интернета.
			monitor := netstatus.StartMonitor(context.Background())

			// Устанавливаем иконку в соответствии с текущим состоянием интернета.
			updateIcon(monitor.Current(context.Background()).Available, offIcoData, unknownIcoData)

			// Обновляем иконку при каждом изменении состояния интернета и
			// управляем опросом ipinfo.io.
			// Внутри колбэка нельзя вызывать monitor.Current (приведёт к deadlock),
			// поэтому используем статус, переданный в аргументе.
			monitor.OnChange(func(s netstatus.Status) {
				// Если данные о стране уже получены — иконку флага не меняем.
				if geo.Received() {
					return
				}
				if s.Available {
					// Интернет появился — запускаем циклический опрос ipinfo.io.
					slog.Info("интернет появился; запущен циклический опрос ipinfo.io")
					updateIcon(true, offIcoData, unknownIcoData)
					geo.startPoll(context.Background(), monitor, onGeoIPReceived)
				} else {
					// Интернет пропал — останавливаем опрос.
					slog.Info("интернет пропал; останавливаем опрос ipinfo.io")
					geo.stopPoll()
					updateIcon(false, offIcoData, unknownIcoData)
				}
			})

			// Если интернет уже доступен при старте — сразу запускаем опрос ipinfo.io.
			if monitor.Current(context.Background()).Available {
				slog.Info("интернет доступен при старте; запущен циклический опрос ipinfo.io")
				geo.startPoll(context.Background(), monitor, onGeoIPReceived)
			} else {
				slog.Info("интернет недоступен при старте")
			}

			quitItem := systray.AddMenuItem("Выход", "Завершить работу программы")
			go func() {
				<-quitItem.ClickedCh
				systray.Quit()
			}()
		},
		func() {
			slog.Info("Программа завершена")
		},
	)
}
