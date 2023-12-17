package rk7api

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"rkeeper2advantshop/pkg/config"
	"rkeeper2advantshop/pkg/logging"
	"rkeeper2advantshop/pkg/rk7api/models"
	"strings"
	"time"
)

const ERROR_TEXT_5310 = "SeqNumber должен быть увеличен."
const ERROS_TEXT_5305 = "В инстансе неверно заполнен атрибут SeqNumber. Атрибут должен именить значение из множества [1, 2]."
const ERROS_NUMBER_5305 = "5305"

type RK7API interface {
	GetRefList() (*models.RK7QueryResultGetRefList, error)
	GetRefData(RefName string, opts ...models.GetRefDataOptions) (RK7QueryResult, error)

	SetRefDataMenuitem(ID int, fields ...models.FieldMenuitemItem) (RK7QueryResult, error)
	SetRefDataMenuitems(menuitemItems []*models.MenuitemItem) (*models.RK7QueryResultSetRefData, error)

	SetRefDataCateglist(categlistItems []*models.Categlist) (*models.RK7QueryResultSetRefData, error)

	GetOrderList() (*models.RK7QueryResultGetOrderList, error)
	GetOrder(Guid string) (*models.RK7QueryResultGetOrder, error)

	CreateOrder(Order *models.OrderInRK7QueryCreateOrder) (*models.RK7QueryResultCreateOrder, error)
	SaveOrder(Visit int, Guid string, Station int, Dishs *[]models.Dish, Prepay *models.Prepay) (*models.RK7QueryResultSaveOrder, error)
	UpdateOrder(Guid string, fields ...models.FieldUpdateOrder) (*models.RK7QueryResultUpdateOrder, error)

	GetXMLLicenseInstanceSeqNumber(Anchor, LicenseToken, Guid string) (*models.RK7QueryResultGetXMLLicenseInstanceSeqNumber, error)
}

var rk7apiGlobal *rk7api

type rk7api struct {
	url     string
	user    string
	pass    string
	xmlType int
	xmlApi  *xmlInterface
}

func (r *rk7api) GetXMLLicenseInstanceSeqNumber(Anchor, LicenseToken, Guid string) (*models.RK7QueryResultGetXMLLicenseInstanceSeqNumber, error) {

	RK7QueryGetXMLLicenseInstanceSeqNumber := new(models.RK7QueryGetXMLLicenseInstanceSeqNumber)
	RK7QueryGetXMLLicenseInstanceSeqNumber.RK7CMD.CMD = "GetXMLLicenseInstanceSeqNumber"
	RK7QueryGetXMLLicenseInstanceSeqNumber.RK7CMD.LicenseInfo.Anchor = Anchor
	RK7QueryGetXMLLicenseInstanceSeqNumber.RK7CMD.LicenseInfo.LicenseToken = LicenseToken
	RK7QueryGetXMLLicenseInstanceSeqNumber.RK7CMD.LicenseInfo.LicenseInstance.Guid = Guid

	xmlQuery, err := xml.MarshalIndent(RK7QueryGetXMLLicenseInstanceSeqNumber, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func GetXMLLicenseInstanceSeqNumber")
	}

	xmlResponse, err := Send(r.url, r.user, r.pass, xmlQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed in func SendToXML")
	}

	RK7QueryResultGetXMLLicenseInstanceSeqNumber := new(models.RK7QueryResultGetXMLLicenseInstanceSeqNumber)
	err = xml.Unmarshal(xmlResponse, RK7QueryResultGetXMLLicenseInstanceSeqNumber)
	if err != nil {
		return nil, errors.Wrap(err, "GetXMLLicenseInstanceSeqNumber>Не удалось выполнить Unmarshal")
	}
	if RK7QueryResultGetXMLLicenseInstanceSeqNumber.XMLName.Local != `RK7QueryResult` {
		return nil, errors.New("Ошибка в Response RK7API.GetOrderList. RK7QueryResult not found")
	}
	if RK7QueryResultGetXMLLicenseInstanceSeqNumber.Status != "Ok" {
		return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.GetOrderList:>%s.%s",
			RK7QueryResultGetXMLLicenseInstanceSeqNumber.Status,
			RK7QueryResultGetXMLLicenseInstanceSeqNumber.ErrorText))
	}

	return RK7QueryResultGetXMLLicenseInstanceSeqNumber, nil
}

func (r *rk7api) UpdateOrder(Guid string, fields ...models.FieldUpdateOrder) (*models.RK7QueryResultUpdateOrder, error) {

	RK7QueryUpdateOrder := new(models.RK7QueryUpdateOrder)
	RK7QueryUpdateOrder.RK7CMD.CMD = "UpdateOrder"
	RK7QueryUpdateOrder.RK7CMD.Order.Guid = Guid

	//add fields is BEAUTIFUL!! BEAUTIFUL!! BEAUTIFUL!!
	for _, field := range fields {
		field(RK7QueryUpdateOrder)
	}

	xmlQuery, err := xml.MarshalIndent(RK7QueryUpdateOrder, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func UpdateOrder")
	}
	cfg := config.GetConfig()
	i := 0
	for {
		xmlResponse, err := Send(r.url, r.user, r.pass, xmlQuery)
		if err != nil {
			return nil, errors.Wrap(err, "failed in func SendToXML")
		}

		rk7QueryResultUpdateOrder := new(models.RK7QueryResultUpdateOrder)
		err = xml.Unmarshal(xmlResponse, rk7QueryResultUpdateOrder)
		if err != nil {
			return nil, errors.Wrap(err, "UpdateOrder:>Не удалось выполнить Unmarshal")
		}
		if rk7QueryResultUpdateOrder.XMLName.Local != `RK7QueryResult` {
			return nil, errors.New("Ошибка в Response RK7API.UpdateOrder. RK7QueryResult not found")
		}
		if rk7QueryResultUpdateOrder.Status != "Ok" {
			if i == 0 {
				time.Sleep(time.Second * time.Duration(cfg.RK7MID.TimeoutError))
				i++
				continue
			}
			return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.UpdateOrder:>%s.%s.%s", rk7QueryResultUpdateOrder.Status, rk7QueryResultUpdateOrder.ErrorText))
		}
		return rk7QueryResultUpdateOrder, nil
	}
}

func (r *rk7api) SetRefDataMenuitem(ID int, fields ...models.FieldMenuitemItem) (RK7QueryResult, error) {

	RK7QuerySetRefDataMenuitems := new(models.RK7QuerySetRefDataMenuitems)
	RK7QuerySetRefDataMenuitems.RK7Command.CMD = "SetRefData"
	RK7QuerySetRefDataMenuitems.RK7Command.RefName = "Menuitems"

	//add fields is BEAUTIFUL!!
	item := new(models.MenuitemItem)
	item.Ident = ID
	for _, field := range fields {
		field(item)
	}
	RK7QuerySetRefDataMenuitems.RK7Command.Items.Item = append(RK7QuerySetRefDataMenuitems.RK7Command.Items.Item, item)

	xmlQuery, err := xml.MarshalIndent(RK7QuerySetRefDataMenuitems, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func SetRefDataMenuitems")
	}

	xmlResponse, err := Send(r.url, r.user, r.pass, xmlQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed in func SendToXML")
	}

	rk7QueryResultSetRefData := new(models.RK7QueryResultSetRefData)
	err = xml.Unmarshal(xmlResponse, rk7QueryResultSetRefData)
	if err != nil {
		return nil, errors.Wrap(err, "SetRefDataMenuitems:>Не удалось выполнить Unmarshal")
	}
	if rk7QueryResultSetRefData.XMLName.Local != `RK7QueryResult` {
		return nil, errors.New("Ошибка в Response RK7API.SetRefData. RK7QueryResult not found")
	}
	if rk7QueryResultSetRefData.Status != "Ok" {
		return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.SetRefData:>%s.%s.%s", rk7QueryResultSetRefData.Status, rk7QueryResultSetRefData.CommandResult.ErrorText, rk7QueryResultSetRefData.ErrorText))
	}
	return rk7QueryResultSetRefData, nil

}

func (r *rk7api) GetOrder(Guid string) (*models.RK7QueryResultGetOrder, error) {
	RK7QueryGetOrder := new(models.RK7QueryGetOrder)
	RK7QueryGetOrder.RK7CMD.CMD = "GetOrder"
	RK7QueryGetOrder.RK7CMD.Guid = Guid

	xmlQuery, err := xml.MarshalIndent(RK7QueryGetOrder, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func GetOrder")
	}

	xmlResponse, err := Send(r.url, r.user, r.pass, xmlQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed in func SendToXML")
	}

	RK7QueryResultGetOrder := new(models.RK7QueryResultGetOrder)
	err = xml.Unmarshal(xmlResponse, RK7QueryResultGetOrder)
	if err != nil {
		return nil, errors.Wrap(err, "GetOrder>Не удалось выполнить Unmarshal")
	}
	if RK7QueryResultGetOrder.XMLName.Local != `RK7QueryResult` {
		return nil, errors.New("Ошибка в Response RK7API.GetOrder. RK7QueryResult not found")
	}
	if RK7QueryResultGetOrder.Status != "Ok" {
		return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.GetOrder:>%s.%s", RK7QueryResultGetOrder.Status, RK7QueryResultGetOrder.ErrorText))
	}

	return RK7QueryResultGetOrder, nil
}

func (r *rk7api) GetOrderList() (*models.RK7QueryResultGetOrderList, error) {
	//todo логирование DEBUG
	RK7QueryGetOrderList := new(models.RK7QueryGetOrderList)
	RK7QueryGetOrderList.RK7CMD.CMD = "GetOrderList"

	xmlQuery, err := xml.MarshalIndent(RK7QueryGetOrderList, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func GetOrderList")
	}

	xmlResponse, err := Send(r.url, r.user, r.pass, xmlQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed in func SendToXML")
	}

	RK7QueryResultGetOrderList := new(models.RK7QueryResultGetOrderList)
	err = xml.Unmarshal(xmlResponse, RK7QueryResultGetOrderList)
	if err != nil {
		return nil, errors.Wrap(err, "GetOrderList>Не удалось выполнить Unmarshal")
	}
	if RK7QueryResultGetOrderList.XMLName.Local != `RK7QueryResult` {
		return nil, errors.New("Ошибка в Response RK7API.GetOrderList. RK7QueryResult not found")
	}
	if RK7QueryResultGetOrderList.Status != "Ok" {
		return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.GetOrderList:>%s.%s", RK7QueryResultGetOrderList.Status, RK7QueryResultGetOrderList.ErrorText))
	}

	return RK7QueryResultGetOrderList, nil
}

func (r *rk7api) SaveOrder(Visit int, Guid string, StationCode int, Dishs *[]models.Dish, Prepay *models.Prepay) (*models.RK7QueryResultSaveOrder, error) {

	logger := logging.GetLogger()
	logger.Println("SaveOrder Start")
	defer logger.Println("SaveOrde End")

	RK7QuerySaveOrder := new(models.RK7QuerySaveOrder)
	RK7QuerySaveOrder.RK7CMD.CMD = "SaveOrder"
	RK7QuerySaveOrder.RK7CMD.Order.Visit = Visit
	RK7QuerySaveOrder.RK7CMD.Order.Guid = Guid
	RK7QuerySaveOrder.RK7CMD.Session.Station.Code = StationCode
	RK7QuerySaveOrder.RK7CMD.Session.Dish = Dishs
	RK7QuerySaveOrder.RK7CMD.Session.Prepay = Prepay

	RK7QueryResultSaveOrder := new(models.RK7QueryResultSaveOrder)
	switch r.xmlType {
	case 1:
		xmlQuery, err := xml.MarshalIndent(RK7QuerySaveOrder, "  ", "    ")
		if err != nil {
			return nil, errors.Wrap(err, "failed Marshal in func RK7QuerySaveOrder")
		}

		xmlResponse, err := Send(r.url, r.user, r.pass, xmlQuery)
		if err != nil {
			return nil, errors.Wrap(err, "failed in func SendToXML")
		}

		err = xml.Unmarshal(xmlResponse, RK7QueryResultSaveOrder)
		if err != nil {
			return nil, errors.Wrap(err, "Не удалось выполнить Unmarshal")
		}
		if RK7QueryResultSaveOrder.XMLName.Local != `RK7QueryResult` {
			return nil, errors.New("Ошибка в Response RK7API.SaveOrder. RK7QueryResult not found")
		}
		if RK7QueryResultSaveOrder.Status != "Ok" {
			return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.SaveOrder:>%s.%s", RK7QueryResultSaveOrder.Status, RK7QueryResultSaveOrder.ErrorText))
		}
	case 2:
		xmlInterface, err := GetXmlInterface()
		if err != nil {
			return nil, errors.Wrap(err, "failed GetXmlInterface()")
		}

		RK7QuerySaveOrder.RK7CMD.LicenseInfo.Anchor = xmlInterface.Anchor
		RK7QuerySaveOrder.RK7CMD.LicenseInfo.LicenseToken = xmlInterface.Id
		RK7QuerySaveOrder.RK7CMD.LicenseInfo.LicenseInstance.Guid = xmlInterface.Guid
		indexRequestNumber := 1 // это чтобы не повторять каждый раз запрос и когда indexRequestNumber++ то будет выход из цикла
		for {
			RK7QuerySaveOrder.RK7CMD.LicenseInfo.LicenseInstance.SeqNumber = xmlInterface.SeqNumber
			xmlQuery, err := xml.MarshalIndent(RK7QuerySaveOrder, "  ", "    ")
			if err != nil {
				return nil, errors.Wrap(err, "failed Marshal in func RK7QuerySaveOrder")
			}

			xmlResponse, err := Send(r.url, r.user, r.pass, xmlQuery)
			if err != nil {
				return nil, errors.Wrap(err, "failed in func SendToXML")
			}

			err = xml.Unmarshal(xmlResponse, RK7QueryResultSaveOrder)
			if err != nil {
				return nil, errors.Wrap(err, "Не удалось выполнить Unmarshal")
			}
			if RK7QueryResultSaveOrder.XMLName.Local != `RK7QueryResult` {
				return nil, errors.New("Ошибка в Response RK7API.SaveOrder. RK7QueryResult not found")
			}

			if RK7QueryResultSaveOrder.Status != "Ok" {
				if (RK7QueryResultSaveOrder.ErrorText == ERROR_TEXT_5310 ||
					RK7QueryResultSaveOrder.ErrorText == ERROS_TEXT_5305 ||
					RK7QueryResultSaveOrder.RK7ErrorN == ERROS_NUMBER_5305) &&
					indexRequestNumber <= 1 {
					logger.Info("Получаем SeqNumber")
					RK7QueryResultGetXMLLicenseInstanceSeqNumber, err := r.GetXMLLicenseInstanceSeqNumber(xmlInterface.Anchor, xmlInterface.Id, xmlInterface.Guid)
					if err != nil {
						return nil, errors.Wrap(err, "failed GetXMLLicenseInstanceSeqNumber")
					}
					xmlInterface.SeqNumber = RK7QueryResultGetXMLLicenseInstanceSeqNumber.LicenseInfo.LicenseInstance.SeqNumber + 1
					indexRequestNumber++
					logger.Info("Повторная отправка")
					continue
				}
				return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.SaveOrder:>%s.%s", RK7QueryResultSaveOrder.Status, RK7QueryResultSaveOrder.ErrorText))
			}
			break
		}
		xmlInterface.SeqNumber++
	default:
		return nil, errors.New(fmt.Sprintf("не удалось определить xmlType=%d", r.xmlType))
	}

	return RK7QueryResultSaveOrder, nil
}

func (r *rk7api) CreateOrder(Order *models.OrderInRK7QueryCreateOrder) (*models.RK7QueryResultCreateOrder, error) {
	RK7QueryCreateOrder := new(models.RK7QueryCreateOrder)
	RK7QueryCreateOrder.RK7CMD.CMD = "CreateOrder"
	RK7QueryCreateOrder.RK7CMD.Order = Order

	xmlQuery, err := xml.MarshalIndent(RK7QueryCreateOrder, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func RK7QueryCreateOrder")
	}

	xmlResponse, err := Send(r.url, r.user, r.pass, xmlQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed in func SendtoXML")
	}

	RK7QueryResultCreateOrder := new(models.RK7QueryResultCreateOrder)
	err = xml.Unmarshal(xmlResponse, RK7QueryResultCreateOrder)
	if err != nil {
		return nil, errors.Wrap(err, "CreateOrder:>Не удалось выполнить Unmarshal")
	}
	if RK7QueryResultCreateOrder.XMLName.Local != `RK7QueryResult` {
		return nil, errors.New("Ошибка в Response RK7API.CreateOrder. RK7QueryResult not found")
	}
	if RK7QueryResultCreateOrder.Status != "Ok" {
		return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.CreateOrder:>%s.%s", RK7QueryResultCreateOrder.Status, RK7QueryResultCreateOrder.ErrorText))
	}
	return RK7QueryResultCreateOrder, nil
}

func (r *rk7api) GetRefData(RefName string, opts ...models.GetRefDataOptions) (RK7QueryResult, error) {
	RK7QueryGetRefData := new(models.RK7QueryGetRefData)
	RK7QueryGetRefData.RK7CMD.CMD = "GetRefData"
	RK7QueryGetRefData.RK7CMD.RefName = RefName

	for _, opt := range opts {
		opt(RK7QueryGetRefData)
	}

	xmlRK7QueryGetRefData, err := xml.MarshalIndent(RK7QueryGetRefData, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func GetRefData")
	}

	xmlRK7QueryResultGetRefData, err := Send(r.url, r.user, r.pass, xmlRK7QueryGetRefData)
	if err != nil {
		return nil, errors.Wrap(err, "failed in func SendToXML")
	}

	switch strings.ToLower(RefName) {
	case "menuitems":
		rk7QueryResultGetRefDataMenuitems := new(models.RK7QueryResultGetRefDataMenuitems)
		err = xml.Unmarshal(xmlRK7QueryResultGetRefData, rk7QueryResultGetRefDataMenuitems)
		if err != nil {
			return nil, errors.Wrap(err, "GetRefData.Menuitems:>Не удалось выполнить Unmarshal")
		}
		if rk7QueryResultGetRefDataMenuitems.XMLName.Local != `RK7QueryResult` {
			return nil, errors.New("Ошибка в Response RK7API.GetRefData. RK7QueryResult not found")
		}
		if rk7QueryResultGetRefDataMenuitems.Status != "Ok" {
			return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.GetRefData:>%s.%s", rk7QueryResultGetRefDataMenuitems.Status, rk7QueryResultGetRefDataMenuitems.ErrorText))
		}
		return rk7QueryResultGetRefDataMenuitems, nil
	case "categlist":
		rk7QueryResultGetRefDataCateglist := new(models.RK7QueryResultGetRefDataCateglist)
		err = xml.Unmarshal(xmlRK7QueryResultGetRefData, rk7QueryResultGetRefDataCateglist)
		if err != nil {
			return nil, errors.Wrap(err, "GetRefData.Categlist:>Не удалось выполнить Unmarshal")
		}
		if rk7QueryResultGetRefDataCateglist.XMLName.Local != `RK7QueryResult` {
			return nil, errors.New("Ошибка в Response RK7API.GetRefData. RK7QueryResult not found")
		}
		if rk7QueryResultGetRefDataCateglist.Status != "Ok" {
			return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.GetRefData:>%s.%s", rk7QueryResultGetRefDataCateglist.Status, rk7QueryResultGetRefDataCateglist.ErrorText))
		}
		return rk7QueryResultGetRefDataCateglist, nil
	default:
		return nil, errors.New(fmt.Sprintf("not found RefName:%s", RefName))
	}
}

func (r *rk7api) GetRefList() (*models.RK7QueryResultGetRefList, error) {
	RK7QueryGetRefList := new(models.RK7QueryGetRefList)
	RK7QueryGetRefList.RK7CMD.CMD = "GetRefList"

	xmlRK7QueryGetRefList, err := xml.MarshalIndent(RK7QueryGetRefList, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func GetOrderList")
	}

	xmlRK7QueryResultGetRefList, err := Send(r.url, r.user, r.pass, xmlRK7QueryGetRefList)
	if err != nil {
		return nil, errors.Wrap(err, "failed in func SendToXML")
	}

	rk7QueryResultGetRefList := new(models.RK7QueryResultGetRefList)
	err = xml.Unmarshal(xmlRK7QueryResultGetRefList, rk7QueryResultGetRefList)
	if err != nil {
		return nil, errors.Wrap(err, "GetRefData.Menuitems:>Не удалось выполнить Unmarshal")
	}
	if rk7QueryResultGetRefList.XMLName.Local != `RK7QueryResult` {
		return nil, errors.New("Ошибка в Response RK7API.GetRefList. RK7QueryResult not found")
	}
	if rk7QueryResultGetRefList.Status != "Ok" {
		return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.GetRefList:>%s.%s", rk7QueryResultGetRefList.Status, rk7QueryResultGetRefList.ErrorText))
	}

	return rk7QueryResultGetRefList, nil
}

func (r *rk7api) SetRefDataMenuitems(menuitemItems []*models.MenuitemItem) (*models.RK7QueryResultSetRefData, error) {
	RK7QuerySetRefDataMenuitems := new(models.RK7QuerySetRefDataMenuitems)
	RK7QuerySetRefDataMenuitems.RK7Command.CMD = "SetRefData"
	RK7QuerySetRefDataMenuitems.RK7Command.RefName = "Menuitems"
	RK7QuerySetRefDataMenuitems.RK7Command.Items.Item = menuitemItems

	xmlRK7QuerySetRefDataMenuitems, err := xml.MarshalIndent(RK7QuerySetRefDataMenuitems, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func SetRefDataMenuitems")
	}

	xmlRK7QueryResultSetRefDataMenuitems, err := Send(r.url, r.user, r.pass, xmlRK7QuerySetRefDataMenuitems)
	if err != nil {
		return nil, errors.Wrap(err, "failed in func SendToXML")
	}

	rk7QueryResultSetRefData := new(models.RK7QueryResultSetRefData)
	err = xml.Unmarshal(xmlRK7QueryResultSetRefDataMenuitems, rk7QueryResultSetRefData)
	if err != nil {
		return nil, errors.Wrap(err, "SetRefDataMenuitems:>Не удалось выполнить Unmarshal")
	}
	if rk7QueryResultSetRefData.XMLName.Local != `RK7QueryResult` {
		return nil, errors.New("Ошибка в Response RK7API.SetRefData. RK7QueryResult not found")
	}
	if rk7QueryResultSetRefData.Status != "Ok" {
		return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.SetRefData:>%s.%s.%s", rk7QueryResultSetRefData.Status, rk7QueryResultSetRefData.CommandResult.ErrorText, rk7QueryResultSetRefData.ErrorText))
	}
	return rk7QueryResultSetRefData, nil
}

func (r *rk7api) SetRefDataCateglist(categlistItems []*models.Categlist) (*models.RK7QueryResultSetRefData, error) {
	RK7QuerySetRefDataCateglist := new(models.RK7QuerySetRefDataCateglist)
	RK7QuerySetRefDataCateglist.RK7Command.CMD = "SetRefData"
	RK7QuerySetRefDataCateglist.RK7Command.RefName = "Categlist"
	RK7QuerySetRefDataCateglist.RK7Command.Items.Item = categlistItems

	xmlRK7QuerySetRefDataCateglist, err := xml.MarshalIndent(RK7QuerySetRefDataCateglist, "  ", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed Marshal in func SetRefDataCateglist")
	}

	xmlRK7QueryResultSetRefDataCateglist, err := Send(r.url, r.user, r.pass, xmlRK7QuerySetRefDataCateglist)
	if err != nil {
		return nil, errors.Wrap(err, "failed in func SendToXML")
	}

	rk7QueryResultSetRefData := new(models.RK7QueryResultSetRefData)
	err = xml.Unmarshal(xmlRK7QueryResultSetRefDataCateglist, rk7QueryResultSetRefData)
	if err != nil {
		return nil, errors.Wrap(err, "SetRefDataCateglist:>Не удалось выполнить Unmarshal")
	}
	if rk7QueryResultSetRefData.XMLName.Local != `RK7QueryResult` {
		return nil, errors.New("Ошибка в Response RK7API.SetRefData. RK7QueryResult not found")
	}
	if rk7QueryResultSetRefData.Status != "Ok" {
		return nil, errors.New(fmt.Sprintf("Ошибка в Response RK7API.SetRefData:>%s.%s.%s", rk7QueryResultSetRefData.Status, rk7QueryResultSetRefData.CommandResult.ErrorText, rk7QueryResultSetRefData.ErrorText))
	}
	return rk7QueryResultSetRefData, nil
}

// Send Отправка запроса в API XML RK7
func Send(url, user, pass string, data []byte) (respBody []byte, e error) {

	logger := logging.GetLogger()
	logger.Println("SendToApiRk7:>Start")
	defer logger.Println("SendToApiRk7:>End")

	logger.Debugf("SendToApiRk7.Request:>\n%s", data)

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10}}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		logger.Infof("SendToApiRk7.NewRequest.ErrorBX24:>%s", err)
		return nil, fmt.Errorf("SendToApiRk7.NewRequest.ErrorBX24:>%s", err)
	}
	req.SetBasicAuth(user, pass)
	req.Header.Add("Content-Type", "application/rk7xml: charset=utf-8")

	resp, err := client.Do(req)
	if err != nil {
		logger.Infof("SendToApiRk7.Do.ErrorBX24:>%s", err)
		return nil, fmt.Errorf("SendToApiRk7.Do.ErrorBX24:>%s", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Error(err)
		}
	}(resp.Body)

	logger.Debugf("SendToApiRk7.Response.Status:>%s", resp.Status)
	logger.Debugf("SendToApiRk7.Response.Header:>%s", resp.Header)

	respBody, err = ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		logger.Infof("SendToApiRk7.ioutil.ReadAll.ErrorBX24:>%s", err)
		return nil, fmt.Errorf("SendToApiRk7.ioutil.ReadAll.ErrorBX24:>%s", err)
	}
	logger.Debugf("SendToApiRk7.Response:>\n%s", respBody)

	return respBody, nil
}

func NewAPI(url string, user string, pass string) (RK7API, error) {

	var err error
	cfg := config.GetConfig()
	xmlApi := new(xmlInterface)

	switch cfg.XMLINTERFACE.Type {
	case 1:
		xmlApi = nil
	case 2:
		xmlApi, err = NewXmlInterface(cfg.XMLINTERFACE.UserName,
			cfg.XMLINTERFACE.Password,
			cfg.XMLINTERFACE.Token,
			cfg.XMLINTERFACE.ProductID,
			cfg.XMLINTERFACE.Guid,
			cfg.XMLINTERFACE.RestCode,
			cfg.XMLINTERFACE.URL,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed NewXmlInterface")
		}
	default:
		return nil, errors.New(fmt.Sprintf("не удалось определить cfg.XMLINTERFACE.Type=%d", cfg.XMLINTERFACE.Type))
	}

	rk7apiGlobal = &rk7api{
		url:     url,
		user:    user,
		pass:    pass,
		xmlType: cfg.XMLINTERFACE.Type,
		xmlApi:  xmlApi,
	}
	return rk7apiGlobal, nil
}

func GetAPI() RK7API {
	return rk7apiGlobal
}
