package f5os

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/json"
	"fmt"
	uuid "github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	productName  string = "terraform-provider-f5os"
	prodEndPoint string = "product.apis.f5.com"
	testEndpoint string = "product-stg.apis.f5networks.net" //"product-tst.apis.f5networks.net"
	ProdKey      string = "mmhJU2sCd63BznXAXDh4kxLIyfIMm3Ar"
	testKey      string = "bWMssM43DzDTX1bXA9CVzdKkOIrk1I8Z"
)

type RawTelemetry struct {
	DigitalAssetId       string                   `json:"digitalAssetId,omitempty"`
	DigitalAssetName     string                   `json:"digitalAssetName,omitempty"`
	DigitalAssetVersion  string                   `json:"digitalAssetVersion,omitempty"`
	DocumentType         string                   `json:"documentType,omitempty"`
	DocumentVersion      string                   `json:"documentVersion,omitempty"`
	EpochTime            int64                    `json:"epochTime,omitempty"`
	ObservationEndTime   string                   `json:"observationEndTime,omitempty"`
	ObservationStartTime string                   `json:"observationStartTime,omitempty"`
	TelemetryRecords     []map[string]interface{} `json:"telemetryRecords,omitempty"`
}

func SendReport(telemetryRecords *RawTelemetry) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}
	url := fmt.Sprintf("https://%s/ee/v1/telemetry", prodEndPoint)
	//url := fmt.Sprintf("https://%s/ee/v1/telemetry", testEndpoint)
	uniqueID := uniqueUUID()
	telemetryRecords.DigitalAssetId = uniqueID
	telemetryRecords.ObservationEndTime = time.Now().UTC().Format(time.RFC3339Nano)
	bodyInfo, _ := json.Marshal(telemetryRecords)
	body := bytes.NewReader([]byte(bodyInfo))
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		fmt.Printf("Error found:%v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("F5-ApiKey", ProdKey)
	//req.Header.Set("F5-ApiKey", testKey)
	req.Header.Set("F5-DigitalAssetId", uniqueID)
	req.Header.Set("F5-TraceId", genUUID())
	f5osLogger.Debug("[SendReport]", "Req :", hclog.Fmt("%+v", req))
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telemetry request to teem server failed with :%v", err)
	}
	f5osLogger.Debug("[SendReport]", "", hclog.Fmt("Resp Code:%+v \t Status:%+v", resp.StatusCode, resp.Status))
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 204 {
		return fmt.Errorf("telemetry request to teem server failed with:%v", string(data[:]))
	}
	return nil
}

func inDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

func genUUID() string {
	id := uuid.New()
	return id.String()
}

var osHostname = os.Hostname

func uniqueUUID() string {
	hostname, err := osHostname()
	hash := md5.New()
	if err != nil {
		return genUUID()
	}
	_, _ = io.WriteString(hash, hostname)
	seed := hash.Sum(nil)
	uid, err := uuid.FromBytes(seed[0:16])
	if err != nil {
		return genUUID()
	}
	result := uid.String()
	return result
}
