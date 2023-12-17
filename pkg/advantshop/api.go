package advantshop

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"net/url"
	"rkeeper2advantshop/pkg/config"
	"rkeeper2advantshop/pkg/logging"
	"rkeeper2advantshop/pkg/telegram"
	"strings"
	"time"
)

const (
	BadRequestError         = 400
	UnauthorizedError       = 401
	NotFoundError           = 404
	InternalServerError     = 500
	MethodNotImplementedErr = 501
)

const (
	Version   = "1.0.0"
	UserAgent = "Advantshop API Client-Golang/" + Version
)

var advantshop *Advantshop

type Advantshop struct {
	Debug            bool            // Is debug mode
	Logger           *logging.Logger // Log
	Services         services        // Advantshop API services
	LastQueryRunTime time.Time
	RPS              int
	ApiKey           string
}

type service struct {
	debug      bool            // Is debug mode
	logger     *logging.Logger // Log
	httpClient *resty.Client   // HTTP client
}

type services struct {
	Orders     OrdersService
	Customers  CustomersService
	Cards      CardsService
	Categories CategoriesService
}

// NewClient - конструктор клиента для Advantshop
func NewClient(config *config.Config) (*Advantshop, error) {
	logger := logging.GetLogger()

	advantshop = &Advantshop{
		Debug:            config.LOG.Debug,
		Logger:           logger,
		ApiKey:           config.ADVANTSHOP.ApiKey,
		LastQueryRunTime: time.Now(),
		RPS:              config.ADVANTSHOP.RPS,
	}

	if config.ADVANTSHOP.Timeout < 2 {
		config.ADVANTSHOP.Timeout = 2
	}

	httpClient := resty.New().
		SetRetryCount(3).
		AddRetryCondition(
			func(r *resty.Response, err error) bool {
				return r.IsError()
			}).
		SetLogger(logger).
		SetDebug(config.LOG.Debug).
		SetBaseURL(strings.TrimRight(config.ADVANTSHOP.URL, "/")).
		SetHeaders(map[string]string{
			"Content-Type": "application/json",
			"Accept":       "text/plain",
			"User-Agent":   UserAgent,
		}).
		SetAllowGetMethodPayload(true).
		SetTimeout(time.Duration(config.ADVANTSHOP.Timeout) * time.Second).
		OnBeforeRequest(func(client *resty.Client, request *resty.Request) (err error) {
			client.SetQueryParam("apikey", advantshop.ApiKey)
			// RPS
			timeSub := time.Now().Sub(advantshop.LastQueryRunTime)
			if timeSub < time.Second/time.Duration(advantshop.RPS) {
				timeSleep := time.Second/time.Duration(advantshop.RPS) - timeSub
				logger.Debugf("timeSub %d nanosecond; sleep %d nanosecond",
					timeSub, timeSleep)
				time.Sleep(timeSleep)
				advantshop.LastQueryRunTime = time.Now()
			}
			return nil
		}).
		OnAfterResponse(func(client *resty.Client, response *resty.Response) (err error) {
			client.QueryParam = url.Values{}
			if response.IsError() {
				logger.Debugf("OnAfterResponse error: %s", err.Error())
				telegram.SendMessageToTelegramWithLogError(fmt.Sprintf("Ошибка при обращении к Advantshop;%s", err.Error()))
			}
			return
		})

	if config.LOG.Debug {
		httpClient.EnableTrace()
	}

	httpClient.JSONMarshal = json.Marshal
	httpClient.JSONUnmarshal = json.Unmarshal
	xService := service{
		debug:      config.LOG.Debug,
		logger:     logger,
		httpClient: httpClient,
	}
	advantshop.Services = services{
		Orders:     (OrdersService)(xService),
		Customers:  (CustomersService)(xService),
		Cards:      (CardsService)(xService),
		Categories: (CategoriesService)(xService),
	}
	return advantshop, nil
}

func GetClient() *Advantshop {
	return advantshop
}
