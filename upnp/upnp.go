package upnp

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/aexel90/fritzbox_exporter/metric"
	"github.com/prometheus/client_golang/prometheus"
)

const text_xml = `text/xml; charset="utf-8"`
const SoapActionParamXML = `<%s>%s</%s>`
const SoapActionXML = `<?xml version="1.0" encoding="utf-8"?>` +
	`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
	`<s:Body><u:%s xmlns:u=%s>%s</u:%s xmlns:u=%s></s:Body>` +
	`</s:Envelope>`

// Exporter struct
type Exporter struct {
	BaseURL  string
	Username string
	Password string
	Device   Device `xml:"device"`
	Services map[string]*Service
}

// An UPNP Device
type Device struct {
	Services         []*Service `xml:"serviceList>service"`
	SubDevices       []*Device  `xml:"deviceList>device"`
	DeviceType       string     `xml:"deviceType"`
	FriendlyName     string     `xml:"friendlyName"`
	Manufacturer     string     `xml:"manufacturer"`
	ManufacturerUrl  string     `xml:"manufacturerURL"`
	ModelDescription string     `xml:"modelDescription"`
	ModelName        string     `xml:"modelName"`
	ModelNumber      string     `xml:"modelNumber"`
	ModelUrl         string     `xml:"modelURL"`
	UDN              string     `xml:"UDN"`
	PresentationUrl  string     `xml:"presentationURL"`
}

// An UPNP Service
type Service struct {
	ServiceType    string             `xml:"serviceType"`
	ServiceId      string             `xml:"serviceId"`
	ControlUrl     string             `xml:"controlURL"`
	EventSubUrl    string             `xml:"eventSubURL"`
	SCPDUrl        string             `xml:"SCPDURL"`
	Actions        map[string]*Action // All actions available on the service
	StateVariables []*StateVariable   // All state variables available on the service
}

// An Argument to an action
type Argument struct {
	Name                 string `xml:"name"`
	Direction            string `xml:"direction"`
	RelatedStateVariable string `xml:"relatedStateVariable"`
	StateVariable        *StateVariable
}

// A state variable that can be manipulated through actions
type StateVariable struct {
	Name         string `xml:"name"`
	DataType     string `xml:"dataType"`
	DefaultValue string `xml:"defaultValue"`
}

// An UPNP Acton on a service
type Action struct {
	service *Service

	Name        string               `xml:"name"`
	Arguments   []*Argument          `xml:"argumentList>argument"`
	ArgumentMap map[string]*Argument // Map of arguments indexed by .Name
}

type scpdRoot struct {
	Actions        []*Action        `xml:"actionList>action"`
	StateVariables []*StateVariable `xml:"serviceStateTable>stateVariable"`
}

type ActionArgument struct {
	Name  string
	Value interface{}
}

// structs to unmarshal SOAP faults
type SoapEnvelope struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	Body    SoapBody
}

type SoapBody struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body"`
	Fault   SoapFault
}
type SoapFault struct {
	XMLName     xml.Name    `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault"`
	FaultCode   string      `xml:"faultcode"`
	FaultString string      `xml:"faultstring"`
	Detail      FaultDetail `xml:"detail"`
}
type FaultDetail struct {
	UpnpError UpnpError `xml:"UPnPError"`
}
type UpnpError struct {
	ErrorCode        int    `xml:"errorCode"`
	ErrorDescription string `xml:"errorDescription"`
}

var authHeader = ""
var (
	collect_errors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fritzbox_exporter_collect_errors",
		Help: "Number of collection errors.",
	})
)

// LoadServices loads the services tree from device
func (exporter *Exporter) LoadServices() error {

	// disable certificate validation, since fritz.box uses self signed cert
	if strings.HasPrefix(exporter.BaseURL, "https://") {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	//igddesc.xml
	err := exporter.load("igddesc.xml")
	if err != nil {
		return err
	}

	// tr64desc.xml
	err = exporter.load("tr64desc.xml")
	if err != nil {
		return err
	}

	// fill services
	exporter.Services = make(map[string]*Service)
	err = exporter.fillServicesForDevice(&exporter.Device)
	if err != nil {
		return err
	}
	return nil
}

func (exporter *Exporter) load(path string) error {

	//HTTP GET
	HTTPResponse, err := http.Get(fmt.Sprintf("%s/%s", exporter.BaseURL, path))
	if err != nil {
		return err
	}
	defer HTTPResponse.Body.Close()

	//decode body
	XMLDecoder := xml.NewDecoder(HTTPResponse.Body)
	err = XMLDecoder.Decode(exporter)
	if err != nil {
		return err
	}
	return nil
}

func (exporter *Exporter) fillServicesForDevice(device *Device) error {

	for _, service := range device.Services {

		HTTPResponse, err := http.Get(exporter.BaseURL + service.SCPDUrl)
		if err != nil {
			return err
		}
		defer HTTPResponse.Body.Close()

		var scpd scpdRoot
		dec := xml.NewDecoder(HTTPResponse.Body)
		err = dec.Decode(&scpd)
		if err != nil {
			return err
		}

		service.StateVariables = scpd.StateVariables
		service.Actions = make(map[string]*Action)
		for _, action := range scpd.Actions {
			service.Actions[action.Name] = action
		}

		for _, action := range service.Actions {
			action.service = service
			action.ArgumentMap = make(map[string]*Argument)

			for _, argument := range action.Arguments {
				for _, stateVariable := range service.StateVariables {
					if argument.RelatedStateVariable == stateVariable.Name {
						argument.StateVariable = stateVariable
					}
				}
				action.ArgumentMap[argument.Name] = argument
			}
		}
		exporter.Services[service.ServiceType] = service
	}
	for _, subDevice := range device.SubDevices {
		err := exporter.fillServicesForDevice(subDevice)
		if err != nil {
			return err
		}
	}
	return nil
}

// Collect func
func (exporter *Exporter) Collect(metrics []*metric.Metric) error {

	var cachedResults = make(map[string]map[string]interface{})

	for _, metric := range metrics {

		metric.MetricResult = nil // remove already collected metrics

		result, err := exporter.request(cachedResults, metric)
		if err != nil {
			fmt.Println(err.Error())
			collect_errors.Inc()
			return err
		}
		metric.MetricResult = result
	}
	return nil
}

func (exporter *Exporter) request(cachedResults map[string]map[string]interface{}, m *metric.Metric) ([]map[string]interface{}, error) {

	var allResults []map[string]interface{}

	var actArg *ActionArgument
	if m.ActionArgument != nil {
		aa := m.ActionArgument
		var value interface{}
		value = aa.Value

		if aa.ProviderAction != "" {
			provRes, err := exporter.GetActionResult(cachedResults, m.Service, aa.ProviderAction, nil)

			if err != nil {
				fmt.Printf("Error getting provider action %s result for %s.%s: %s\n", aa.ProviderAction, m.Service, m.Action, err.Error())
				collect_errors.Inc()
				// continue
			}

			var ok bool
			value, ok = provRes[aa.Value] // Value contains the result name for provider actions
			if !ok {
				fmt.Printf("provider action %s for %s.%s has no result %s", m.Service, m.Action, aa.Value)
				collect_errors.Inc()
				// continue
			}
		}

		if aa.IsIndex {
			sval := fmt.Sprintf("%v", value)
			count, err := strconv.Atoi(sval)
			if err != nil {
				fmt.Println(err.Error())
				collect_errors.Inc()
				// continue
			}

			for i := 0; i < count; i++ {
				actArg = &ActionArgument{Name: aa.Name, Value: i}
				result, err := exporter.GetActionResult(cachedResults, m.Service, m.Action, actArg)

				if err != nil {
					fmt.Println(err.Error())
					collect_errors.Inc()
					continue
				}

				// fc.ReportMetric(ch, m, result)
				allResults = append(allResults, result)
			}

			//continue
		} else {
			actArg = &ActionArgument{Name: aa.Name, Value: value}
		}
	} else {

		result, err := exporter.GetActionResult(cachedResults, m.Service, m.Action, actArg)

		if err != nil {
			fmt.Println(err.Error())
			collect_errors.Inc()
			// continue
		}

		// fc.ReportMetric(ch, m, result)
		//fmt.Println(result)
		allResults = append(allResults, result)
	}
	return allResults, nil
}

func (exporter *Exporter) GetActionResult(result_map map[string]map[string]interface{}, serviceType string, actionName string, actionArg *ActionArgument) (map[string]interface{}, error) {

	m_key := serviceType + "|" + actionName

	// for calls with argument also add arguement name and value to key
	if actionArg != nil {

		m_key += "|" + actionArg.Name + "|" + fmt.Sprintf("%v", actionArg.Value)
	}

	last_result := result_map[m_key]
	if last_result == nil {
		service, ok := exporter.Services[serviceType]
		if !ok {
			return nil, errors.New(fmt.Sprintf("service %s not found", serviceType))
		}

		action, ok := service.Actions[actionName]
		if !ok {
			return nil, errors.New(fmt.Sprintf("action %s not found in service %s", actionName, serviceType))
		}

		var err error
		last_result, err = exporter.Call(action, actionArg)

		if err != nil {
			return nil, err
		}

		result_map[m_key] = last_result
	}

	return last_result, nil
}

// Call an action with argument if given
func (exporter *Exporter) Call(a *Action, actionArg *ActionArgument) (map[string]interface{}, error) {
	req, err := exporter.createCallHttpRequest(a, actionArg)

	if err != nil {
		return nil, err
	}

	// reuse prior authHeader, to avoid unnecessary authentication
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	// first try call without auth header
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, err
	}

	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close() // close now, since we make a new request below or fail

		if wwwAuth != "" && exporter.Username != "" && exporter.Password != "" {
			// call failed, but we have a password so calculate header and try again
			authHeader, err = exporter.getDigestAuthHeader(a, wwwAuth, exporter.Username, exporter.Password)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("%s: %s", a.Name, err.Error))
			}

			req, err = exporter.createCallHttpRequest(a, actionArg)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("%s: %s", a.Name, err.Error))
			}

			req.Header.Set("Authorization", authHeader)

			resp, err = http.DefaultClient.Do(req)

			if err != nil {
				return nil, errors.New(fmt.Sprintf("%s: %s", a.Name, err.Error))
			}

		} else {
			return nil, errors.New(fmt.Sprintf("%s: Unauthorized, but no username and password given", a.Name))
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("%s (%d)", http.StatusText(resp.StatusCode), resp.StatusCode)
		if resp.StatusCode == 500 {
			buf := new(strings.Builder)
			io.Copy(buf, resp.Body)
			body := buf.String()
			//fmt.Println(body)

			var soapEnv SoapEnvelope
			err := xml.Unmarshal([]byte(body), &soapEnv)
			if err != nil {
				errMsg = fmt.Sprintf("error decoding SOAPFault: %s", err.Error())
			} else {
				soapFault := soapEnv.Body.Fault

				if soapFault.FaultString == "UPnPError" {
					upe := soapFault.Detail.UpnpError

					errMsg = fmt.Sprintf("SAOPFault: %s %d (%s)", soapFault.FaultString, upe.ErrorCode, upe.ErrorDescription)
				} else {
					errMsg = fmt.Sprintf("SAOPFault: %s", soapFault.FaultString)
				}
			}
		}
		return nil, errors.New(fmt.Sprintf("%s: %s", a.Name, errMsg))
	}

	return a.parseSoapResponse(resp.Body)
}

func (exporter *Exporter) getDigestAuthHeader(a *Action, wwwAuth string, username string, password string) (string, error) {
	// parse www-auth header
	if !strings.HasPrefix(wwwAuth, "Digest ") {
		return "", errors.New(fmt.Sprintf("WWW-Authentication header is not Digest: '%s'", wwwAuth))
	}

	s := wwwAuth[7:]
	d := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		d[strings.Trim(parts[0], "\" ")] = strings.Trim(parts[1], "\" ")
	}

	if d["algorithm"] == "" {
		d["algorithm"] = "MD5"
	} else if d["algorithm"] != "MD5" {
		return "", errors.New(fmt.Sprintf("digest algorithm not supported: %s != MD5", d["algorithm"]))
	}

	if d["qop"] != "auth" {
		return "", errors.New(fmt.Sprintf("digest qop not supported: %s != auth", d["qop"]))
	}

	// calc h1 and h2
	ha1 := fmt.Sprintf("%x", md5.Sum([]byte(username+":"+d["realm"]+":"+password)))

	ha2 := fmt.Sprintf("%x", md5.Sum([]byte("POST:"+a.service.ControlUrl)))

	cn := make([]byte, 8)
	rand.Read(cn)
	cnonce := fmt.Sprintf("%x", cn)

	nCounter := 1
	nc := fmt.Sprintf("%08x", nCounter)

	ds := strings.Join([]string{ha1, d["nonce"], nc, cnonce, d["qop"], ha2}, ":")
	response := fmt.Sprintf("%x", md5.Sum([]byte(ds)))

	authHeader := fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", cnonce=\"%s\", nc=%s, qop=%s, response=\"%s\", algorithm=%s",
		username, d["realm"], d["nonce"], a.service.ControlUrl, cnonce, nc, d["qop"], response, d["algorithm"])

	return authHeader, nil
}

func (exporter *Exporter) createCallHttpRequest(a *Action, actionArg *ActionArgument) (*http.Request, error) {
	argsString := ""
	if actionArg != nil {
		var buf bytes.Buffer
		sValue := fmt.Sprintf("%v", actionArg.Value)
		xml.EscapeText(&buf, []byte(sValue))
		argsString += fmt.Sprintf(SoapActionParamXML, actionArg.Name, buf.String(), actionArg.Name)
	}
	bodystr := fmt.Sprintf(SoapActionXML, a.Name, a.service.ServiceType, argsString, a.Name, a.service.ServiceType)

	url := exporter.BaseURL + a.service.ControlUrl
	body := strings.NewReader(bodystr)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	action := fmt.Sprintf("%s#%s", a.service.ServiceType, a.Name)

	req.Header.Set("Content-Type", text_xml)
	req.Header.Set("SOAPAction", action)

	return req, nil
}

func (a *Action) parseSoapResponse(r io.Reader) (map[string]interface{}, error) {
	res := make(map[string]interface{})
	dec := xml.NewDecoder(r)

	for {
		t, err := dec.Token()
		if err == io.EOF {
			return res, nil
		}

		if err != nil {
			return nil, err
		}

		if se, ok := t.(xml.StartElement); ok {
			arg, ok := a.ArgumentMap[se.Name.Local]

			if ok {
				t2, err := dec.Token()
				if err != nil {
					return nil, err
				}

				var val string
				switch element := t2.(type) {
				case xml.EndElement:
					val = ""
				case xml.CharData:
					val = string(element)
				default:
					return nil, errors.New("invalid SOAP response")
				}

				converted, err := convertResult(val, arg)
				if err != nil {
					return nil, err
				}
				res[arg.StateVariable.Name] = converted
			}
		}

	}
}

func convertResult(val string, arg *Argument) (interface{}, error) {
	switch arg.StateVariable.DataType {
	case "string":
		return val, nil
	case "boolean":
		return bool(val == "1"), nil

	case "ui1", "ui2", "ui4":
		// type ui4 can contain values greater than 2^32!
		res, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return nil, err
		}
		return uint64(res), nil
	case "i4":
		res, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, err
		}
		return int64(res), nil
	case "dateTime", "uuid":
		// data types we don't convert yet
		return val, nil
	default:
		return nil, fmt.Errorf("unknown datatype: %s (%s)", arg.StateVariable.DataType, val)
	}
}
