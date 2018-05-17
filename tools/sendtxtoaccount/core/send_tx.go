package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bytom/api"
	"github.com/bytom/blockchain/txbuilder"
	"github.com/bytom/protocol/bc/types"
)

const (
	BuildMulTx = "build_mul_tx"
	SignTx     = "Sign_tx"
	SubmitTx   = "submit_tx"
)

var actions = `{"actions": [%s]}`
var feesFmt = `{"type": "spend_account", "asset_id": "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", "amount":%s, "account_id": "%s"}`
var inputFmt = `{"type": "spend_account", "asset_id": "%s","amount": %s,"account_id": "%s"}`
var issueInputFmt = `{"type": "issue", "asset_id": "%s", "amount": %s}`
var outputFmt = `{"type": "control_address", "asset_id": "%s", "amount": %s,"address": "%s"}`

var (
	buildType = ""
	btmGas    = "20000000"
	passwd    = "123456"
	baseNum   = 100000000
	index     = 0
	password  = ""
)

// SendReq genetate tx and send data
func SendReq(method string, args []string, recvAccount []accountInfo) (interface{}, bool) {
	var param interface{}
	var methodPath string
	switch method {
	case BuildMulTx:
		// send account
		accountInfo := args[0]
		// send btm asset
		assetInfo := args[1]
		bmtTotalAmount := uint64(0)
		var (
			input  string
			fees   string
			output string
		)
		// generate output data
		for i := 0; i < len(recvAccount); i++ {
			address := recvAccount[i].address
			bmtTotalAmount += recvAccount[i].amount
			amountTmp := strconv.FormatUint(recvAccount[i].amount, 10)
			output += fmt.Sprintf(outputFmt, assetInfo, amountTmp, address)
			output += ","
		}
		amountTmp := strconv.FormatUint(bmtTotalAmount, 10)
		btmGasTmp := cfg.BtmGas
		btmGas = strconv.Itoa(int(btmGasTmp))
		fees += fmt.Sprintf(feesFmt, btmGas, accountInfo) + ","
		input += fmt.Sprintf(inputFmt, assetInfo, amountTmp, accountInfo)

		buildReqStr := fmt.Sprintf(actions, fees+output+input)
		var ins api.BuildRequest
		if err := json.Unmarshal([]byte(buildReqStr), &ins); err != nil {
			fmt.Println("generate build mul tx is error: ", err)
			os.Exit(ErrLocalExe)
		}

		rawData, err := json.MarshalIndent(&ins, "", "  ")
		if err != nil {
			fmt.Println("Json format error!!!!!")
			os.Exit(1)
		}

		fmt.Println(string(rawData))
		fmt.Println("The total number of btm[neu]:", bmtTotalAmount)
		param = ins
		methodPath = "/build-transaction"

	case SignTx:
		template := txbuilder.Template{}

		err := json.Unmarshal([]byte(args[0]), &template)
		if err != nil {
			fmt.Println(err)
			os.Exit(ErrLocalExe)
		}
		if len(password) == 0 {
			fmt.Println("password is null")
			os.Exit(1)
		}
		var ins = struct {
			Password string             `json:"password"`
			Txs      txbuilder.Template `json:"transaction"`
		}{Password: password, Txs: template}
		param = ins
		methodPath = "/sign-transaction"
	case SubmitTx:
		var ins = struct {
			Tx types.Tx `json:"raw_transaction"`
		}{}
		json.Unmarshal([]byte(args[0]), &ins)
		methodPath = "/submit-transaction"
		data, exitCode := ClientCall(methodPath, &ins)
		if exitCode != Success {
			return "", false
		}
		return data, true
	default:
		fmt.Println("method is null")
		os.Exit(1)
	}
	data, exitCode := ClientCall(methodPath, &param)
	if exitCode != Success {
		return "", false
	}
	return data, true
}

// Sendbulktx send asset tx
func Sendtx(sendAcct string, sendasset string, recvAccount []accountInfo) {
	//build tx
	var (
		resp interface{}
		b    bool
	)
	param := []string{sendAcct, sendasset}
	resp, b = SendReq(BuildMulTx, param, recvAccount)
	if !b {
		fmt.Println("BuildMulTx fail!")
		os.Exit(1)
	}
	{
		fileName := "build_tx_" + strconv.Itoa(index) + ".txt"
		outputFile, outputError := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
		if outputError != nil {
			fmt.Println("Failed to open file:", fileName, ",Please check the file.If it exists, please backup.")
			os.Exit(1)
		}
		defer outputFile.Close()
		outputWriter := bufio.NewWriter(outputFile)
		dataMap, _ := resp.(map[string]interface{})
		rawData, _ := json.MarshalIndent(dataMap, "", "  ")
		outputWriter.WriteString(string(rawData))
		outputWriter.Flush()
		fmt.Println("\n\n", string(rawData), "\n")
		fmt.Println("Please check the above data or file:[", fileName, "] data")

	}
	if cfg.OnlyBuildTx {
		index += 1
		return
	}
	rawTemplate, _ := json.Marshal(resp)
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("The transaction will be signed, please confirm whether you want to continue.")
	fmt.Printf("Enter yes or no after checking:")
	data, _, _ := reader.ReadLine()
	command := strings.ToLower(strings.TrimSpace(string(data)))
	if command == "yes" {
		tmp := make([]accountInfo, 0)
		//sign
		fmt.Printf("Please enter your password:")
		pswd, _, _ := reader.ReadLine()
		password = strings.TrimSpace(string(pswd))
		param = []string{string(rawTemplate)}
		resp, b = SendReq(SignTx, param, tmp)
		if !b {
			fmt.Println("SignTx fail!")
			os.Exit(1)
		}
		dataMap, _ := resp.(map[string]interface{})
		rawData, _ := json.MarshalIndent(dataMap, "", "  ")
		rawTemplate, _ = json.Marshal(resp)
		fmt.Println("\n\n", string(rawData), "\n")
		fmt.Println("Will broadcast the transaction, please confirm if you want to continue.")
		fmt.Printf("Enter yes or no after checking:")
		data, _, _ := reader.ReadLine()
		command := strings.ToLower(strings.TrimSpace(string(data)))
		if command == "yes" {
			// submit
			var data signResp
			json.Unmarshal(rawTemplate, &data)
			rawTemplate, _ = json.Marshal(*data.Tx)
			param = []string{string(rawTemplate)}

			resp, b = SendReq(SubmitTx, param, tmp)
			if !b {
				fmt.Println("SubmitTx fail!")
				os.Exit(1)
			}
			index += 1
			fmt.Println("\n\n", resp)
		} else {
			fmt.Println("exit...")
			os.Exit(1)
		}

	} else {
		fmt.Println("exit...")
		os.Exit(1)
	}
}