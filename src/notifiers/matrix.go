package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type MatrixSender struct {
	tag string
	// only used for discord
	matrixUrl string
	// Level
	logLevel string
}

func NewMatrixSender(matrixConf map[string]string, confName string) (*MatrixSender, string) {
	curError := ""
	matrixSender := &MatrixSender{}
	if tag, ok := matrixConf["TAG"]; ok {
		matrixSender.tag = tag
	} else {
		curError += fmt.Sprintf("missing 'TAG' option for config '%s'\n", confName)
	}
	if matrixUrl, ok := matrixConf["URL"]; ok {
		matrixSender.matrixUrl = matrixUrl
	} else {
		curError += fmt.Sprintf("missing 'URL' option for config '%s'\n", confName)
	}
	if logLevel, ok := matrixConf["LOGLEVEL"]; ok {
		matrixSender.logLevel = logLevel
	} else {
		curError += fmt.Sprintf("missing 'LOGLEVEL' option for config '%s'\n", confName)
	}
	if curError == "" {
		return matrixSender, ""
	}
	return nil, curError
}

func (matrixSender *MatrixSender) SendMessage(msg, notUsed, notUsedAlso string) error {
	jsonByteMessage, err := json.Marshal(map[string]string{"content": msg})
	if err != nil {
		return fmt.Errorf("failed to marshall message (matrix): %v", err)
	}
	httpsResp, err := http.Post(matrixSender.matrixUrl, "application/json", bytes.NewBuffer(jsonByteMessage))
	if err != nil {
		return fmt.Errorf("failed to post http request (matrix): %v", err)
	}
	defer httpsResp.Body.Close()
	if httpsResp.StatusCode != http.StatusOK {
		return fmt.Errorf(fmt.Sprintf("unexpected http status (matrix): %s", httpsResp.StatusCode))
	}
	return nil
}

func (m *MatrixSender) GetTag() string {
	return m.tag
}

func (m *MatrixSender) GetLogLevel() string {
	return m.logLevel
}
