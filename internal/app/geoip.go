package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/iamcalledrob/netstatus"
	"github.com/kirsrus/ip2flag/internal/model"
)

// ipinfoURL — адрес сервиса определения местоположения по IP.
const ipinfoURL = "https://ipinfo.io"

// ipinfoPollInterval — пауза между повторными попытками опроса ipinfo.io.
const ipinfoPollInterval = 5 * time.Second

// ipinfoHTTPTimeout — таймаут одного HTTP-запроса к ipinfo.io.
const ipinfoHTTPTimeout = 10 * time.Second

// errEmptyCountry — ошибка, когда сервис не вернул поле country.
var errEmptyCountry = errors.New("ipinfo.io не вернул поле country")

// ipinfoClient — HTTP-клиент для опроса ipinfo.io.
var ipinfoClient = &http.Client{Timeout: ipinfoHTTPTimeout}

// geoState хранит результат опроса ipinfo.io и управляет циклом опроса.
type geoState struct {
	mu     sync.Mutex
	geo    *model.GeoIP
	cancel context.CancelFunc
}

// Received сообщает, получены ли уже данные от ipinfo.io.
func (s *geoState) Received() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.geo != nil
}

// startPoll запускает циклический опрос ipinfo.io. Если опрос уже идёт или
// данные уже получены, вызов игнорируется. После успешного получения данных
// вызывается onGeo, а опрос останавливается.
func (s *geoState) startPoll(parent context.Context, monitor *netstatus.Monitor, onGeo func(*model.GeoIP)) {
	s.mu.Lock()
	if s.geo != nil || s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.mu.Unlock()

	go func() {
		// Останавливаем опрос при любом завершении горутины:
		// после получения данных или при потере сети.
		defer s.stopPoll()

		geo := pollGeoIP(ctx, monitor)
		if geo == nil {
			return // опрос остановлен из-за потери сети или отмены
		}

		s.mu.Lock()
		s.geo = geo
		s.mu.Unlock()

		onGeo(geo)
	}()
}

// stopPoll останавливает цикл опроса ipinfo.io, если он запущен.
func (s *geoState) stopPoll() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// pollGeoIP циклически опрашивает сервис ipinfo.io до получения корректного
// ответа. Возвращает nil, если опрос остановлен: контекст отменён или сеть
// пропала.
func pollGeoIP(ctx context.Context, monitor *netstatus.Monitor) *model.GeoIP {
	ticker := time.NewTicker(ipinfoPollInterval)
	defer ticker.Stop()

	for {
		// Опрос прекращается, если сеть пропала.
		if !monitor.Current(ctx).Available {
			return nil
		}

		geo, err := fetchGeoIP(ctx)
		if err == nil {
			return geo
		}
		slog.Warn("не удалось получить данные от ipinfo.io", slog.String("error", err.Error()))

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// fetchGeoIP выполняет один запрос к ipinfo.io и разбирает ответ в model.GeoIP.
func fetchGeoIP(ctx context.Context) (*model.GeoIP, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ipinfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ip2flag/"+version)

	resp, err := ipinfoClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ipinfo.io ответил статусом %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var geo model.GeoIP
	if err := json.Unmarshal(body, &geo); err != nil {
		return nil, fmt.Errorf("не удалось разобрать ответ ipinfo.io: %w", err)
	}
	if strings.TrimSpace(geo.Country) == "" {
		return nil, errEmptyCountry
	}
	return &geo, nil
}
