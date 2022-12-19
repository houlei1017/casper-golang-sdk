package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/casper-ecosystem/casper-golang-sdk/bigint"
	"io/ioutil"
	"math/big"
	"net/http"
	"reflect"
	"time"

	"github.com/casper-ecosystem/casper-golang-sdk/keypair"

	"github.com/pkg/errors"
)

type RpcClient struct {
	endpoint string
}

func NewRpcClient(endpoint string) *RpcClient {
	return &RpcClient{
		endpoint: endpoint,
	}
}

func (c *RpcClient) GetDeploy(hash string) (DeployResult, error) {
	resp, err := c.rpcCall("info_get_deploy", map[string]string{
		"deploy_hash": hash,
	})
	if err != nil {
		//交易不存在
		if resp.Error.Code == -32000 {
			return DeployResult{}, nil
		} else {
			return DeployResult{}, err
		}
	}

	var result DeployResult
	err = json.Unmarshal(resp.Result, &result)

	if err != nil {
		return DeployResult{}, fmt.Errorf("failed to get result: %w", err)
	}

	return result, nil
}

func (c *RpcClient) GetStateItem(stateRootHash, key string, path []string) (StoredValue, error) {
	params := map[string]interface{}{
		"state_root_hash": stateRootHash,
		"key":             key,
	}
	if len(path) > 0 {
		params["path"] = path
	}
	resp, err := c.rpcCall("state_get_item", params)
	if err != nil {
		return StoredValue{}, err
	}

	var result storedValueResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return StoredValue{}, fmt.Errorf("failed to get result: %w", err)
	}

	return result.StoredValue, nil
}

func (c *RpcClient) GetAccountBalance(stateRootHash, balanceUref string) (*big.Int, error) {
	resp, err := c.rpcCall("state_get_balance", map[string]string{
		"state_root_hash": stateRootHash,
		"purse_uref":      balanceUref,
	})
	if err != nil {
		return nil, err
	}

	var result balanceResponse
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	balance := big.NewInt(0)
	balance.SetString(result.BalanceValue, 10)
	return balance, nil
}

func (c *RpcClient) GetLiquidBalance(publicKey string) (*big.Int, error) {
	//1 获取 state_root_hash
	stateRootHash, err := c.GetStateRootHash()
	if err != nil {
		return nil, err
	}

	//2 获取 purse_uref
	accountInfo, err := c.GetAccountInfo(publicKey)
	if err != nil {
		return nil, err
	}
	//如果返回的是空数据，即用户数据未上链
	if reflect.DeepEqual(accountInfo, AccountInfo{}) {
		return big.NewInt(0), nil
	}

	//3 获取账户的流动资产
	return c.GetAccountBalance(stateRootHash.StateRootHash, accountInfo.Account.MainPurse)
}

func (c *RpcClient) GetStackingBalance(publicKey string) ([]StackInfo, error) {
	ValidatorInfo, err := c.GetValidator()
	if err != nil {
		return nil, err
	}

	infolist := make([]StackInfo, 0, 10)
	for i := range ValidatorInfo.AuctionState.Bids {
		delegators := ValidatorInfo.AuctionState.Bids[i].Bid.Delegators
		for j := range delegators {
			if delegators[j].PublicKey == publicKey {
				infolist = append(infolist, delegators[j])
			}
		}
	}

	return infolist, nil
}

func (c *RpcClient) GetAccountMainPurseURef(accountHash string) string {
	block, err := c.GetLatestBlock()
	if err != nil {
		return ""
	}

	item, err := c.GetStateItem(block.Header.StateRootHash, accountHash, []string{})
	if err != nil {
		return ""
	}

	return item.Account.MainPurse
}

func (c *RpcClient) GetAccountBalanceByKeypair(stateRootHash string, key keypair.KeyPair) (*big.Int, error) {
	item, err := c.GetStateItem(stateRootHash, key.AccountHash(), []string{})
	if err != nil {
		return nil, err
	}
	return c.GetAccountBalance(stateRootHash, item.Account.MainPurse)
}

func (c *RpcClient) GetLatestBlock() (BlockResponse, error) {
	resp, err := c.rpcCall("chain_get_block", nil)
	if err != nil {
		return BlockResponse{}, err
	}

	var result blockResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return BlockResponse{}, fmt.Errorf("failed to get result: %w", err)
	}

	return result.Block, nil
}

func (c *RpcClient) GetBlockByHeight(height uint64) (BlockResponse, error) {
	resp, err := c.rpcCall("chain_get_block",
		blockParams{blockIdentifier{
			Height: height,
		}})
	if err != nil {
		return BlockResponse{}, err
	}

	var result blockResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return BlockResponse{}, fmt.Errorf("failed to get result: %w", err)
	}

	return result.Block, nil
}

func (c *RpcClient) GetBlockByHash(hash string) (BlockResponse, error) {
	resp, err := c.rpcCall("chain_get_block",
		blockParams{blockIdentifier{
			Hash: hash,
		}})
	if err != nil {
		return BlockResponse{}, err
	}

	var result blockResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return BlockResponse{}, fmt.Errorf("failed to get result: %w", err)
	}

	return result.Block, nil
}

func (c *RpcClient) GetLatestBlockTransfers() (*transferResult, error) {
	resp, err := c.rpcCall("chain_get_block_transfers", nil)
	if err != nil {
		return nil, err
	}

	var result transferResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	return &result, nil
}

func (c *RpcClient) GetBlockTransfersByHeight(height uint64) (*transferResult, error) {
	resp, err := c.rpcCall("chain_get_block_transfers",
		blockParams{blockIdentifier{
			Height: height,
		}})
	if err != nil {
		return nil, err
	}

	var result transferResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	return &result, nil
}

func (c *RpcClient) GetBlockTransfersByHash(blockHash string) (*transferResult, error) {
	resp, err := c.rpcCall("chain_get_block_transfers",
		blockParams{blockIdentifier{
			Hash: blockHash,
		}})
	if err != nil {
		return nil, err
	}

	var result transferResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	return &result, nil
}

func (c *RpcClient) GetValidator() (ValidatorPesponse, error) {
	resp, err := c.rpcCall("state_get_auction_info", nil)
	if err != nil {
		return ValidatorPesponse{}, err
	}

	var result ValidatorPesponse
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return ValidatorPesponse{}, fmt.Errorf("failed to get result: #{err}")
	}

	return result, nil
}

func (c *RpcClient) GetStatus() (StatusResult, error) {
	resp, err := c.rpcCall("info_get_status", nil)
	if err != nil {
		return StatusResult{}, err
	}

	var result StatusResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return StatusResult{}, fmt.Errorf("failed to get result: #{err}")
	}

	return result, nil
}

func (c *RpcClient) GetPeers() (PeerResult, error) {
	resp, err := c.rpcCall("info_get_peers", nil)
	if err != nil {
		return PeerResult{}, err
	}

	var result PeerResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return PeerResult{}, fmt.Errorf("failed to get result: #{err}")
	}

	return result, nil
}

func (c *RpcClient) GetStateRootHash() (StateRootHashResult, error) {
	resp, err := c.rpcCall("chain_get_state_root_hash", nil)
	if err != nil {
		return StateRootHashResult{}, err
	}

	var result StateRootHashResult
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return StateRootHashResult{}, fmt.Errorf("failed to get result: %w", err)
	}

	return result, nil
}

func (c *RpcClient) GetAccountInfo(publicKey string) (AccountInfo, error) {
	resp, err := c.rpcCall("state_get_account_info", map[string]interface{}{
		"public_key": publicKey,
	})
	if err != nil {
		//账户未上链
		if resp.Error.Code == -32003 {
			return AccountInfo{}, nil
		} else {
			return AccountInfo{}, err
		}
	}

	var result AccountInfo
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return AccountInfo{}, fmt.Errorf("failed to get result: %w", err)
	}

	return result, nil
}

func (c *RpcClient) PutDeploy(deploy Deploy) (JsonPutDeployRes, error) {
	resp, err := c.rpcCall("account_put_deploy", map[string]interface{}{
		"deploy": deploy,
	})

	if err != nil {
		return JsonPutDeployRes{}, err
	}

	var result JsonPutDeployRes
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		return JsonPutDeployRes{}, fmt.Errorf("failed to put deploy: %w", err)
	}

	return result, nil
}

func (c *RpcClient) rpcCall(method string, params interface{}) (RpcResponse, error) {
	body, err := json.Marshal(RpcRequest{
		Version: "2.0",
		Method:  method,
		Params:  params,
	})

	if err != nil {
		return RpcResponse{}, errors.Wrap(err, "failed to marshal json")
	}

	resp, err := http.Post(c.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return RpcResponse{}, fmt.Errorf("failed to make request: %w", err)
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return RpcResponse{}, fmt.Errorf("failed to get response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return RpcResponse{Error: &RpcError{Code: resp.StatusCode, Message: string(b)}},
			fmt.Errorf("request failed, status code - %d, response - %s", resp.StatusCode, string(b))
	}

	var rpcResponse RpcResponse
	err = json.Unmarshal(b, &rpcResponse)
	if err != nil {
		return RpcResponse{}, fmt.Errorf("failed to parse response body: %w", err)
	}

	if rpcResponse.Error != nil {
		return rpcResponse, fmt.Errorf("rpc call failed, code - %d, message - %s", rpcResponse.Error.Code, rpcResponse.Error.Message)
	}

	return rpcResponse, nil
}

type RpcRequest struct {
	Version string      `json:"jsonrpc"`
	Id      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type RpcResponse struct {
	Version string          `json:"jsonrpc"`
	Id      string          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *RpcError       `json:"error,omitempty"`
}

type RpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type transferResult struct {
	Transfers []TransferResponse `json:"transfers"`
	BlockHash string             `json:"block_hash"`
}

type TransferResponse struct {
	ID         uint64 `json:"id,omitempty"`
	DeployHash string `json:"deploy_hash"`
	From       string `json:"from"`
	To         string `json:"to"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	Amount     string `json:"amount"`
	Gas        string `json:"gas"`
}

type blockResult struct {
	Block BlockResponse `json:"block"`
}

type BlockResponse struct {
	Hash   string      `json:"hash"`
	Header BlockHeader `json:"header"`
	Body   BlockBody   `json:"body"`
	Proofs []Proof     `json:"proofs"`
}

type BlockInfo struct {
	Hash          string    `json:"hash"`
	Timestamp     time.Time `json:"timestamp"`
	EraId         int       `json:"era_id"`
	Height        int       `json:"height"`
	StateRootHash string    `json:"state_root_hash"`
	Creator       string    `json:"creator"`
}

type BlockHeader struct {
	ParentHash      string    `json:"parent_hash"`
	StateRootHash   string    `json:"state_root_hash"`
	BodyHash        string    `json:"body_hash"`
	RandomBit       bool      `json:"random_bit"`
	AccumulatedSeed string    `json:"accumulated_seed"`
	Timestamp       time.Time `json:"timestamp"`
	EraID           int       `json:"era_id"`
	Height          int       `json:"height"`
	ProtocolVersion string    `json:"protocol_version"`
}

type BlockBody struct {
	Proposer       string   `json:"proposer"`
	DeployHashes   []string `json:"deploy_hashes"`
	TransferHashes []string `json:"transfer_hashes"`
}

type Proof struct {
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

type DeployResult struct {
	Deploy           JsonDeploy            `json:"deploy"`
	ExecutionResults []JsonExecutionResult `json:"execution_results"`
}

type JsonDeploy struct {
	Hash      string           `json:"hash"`
	Header    JsonDeployHeader `json:"header"`
	Approvals []JsonApproval   `json:"approvals"`
	Session   Session          `json:"session"`
}

type JsonPutDeployRes struct {
	Hash string `json:"deploy_hash"`
}

type JsonDeployHeader struct {
	Account      string    `json:"account"`
	Timestamp    time.Time `json:"timestamp"`
	TTL          string    `json:"ttl"`
	GasPrice     int       `json:"gas_price"`
	BodyHash     string    `json:"body_hash"`
	Dependencies []string  `json:"dependencies"`
	ChainName    string    `json:"chain_name"`
}

type JsonApproval struct {
	Signer    string `json:"signer"`
	Signature string `json:"signature"`
}

type JsonExecutionResult struct {
	BlockHash string          `json:"block_hash"`
	Result    ExecutionResult `json:"result"`
}

type ExecutionResult struct {
	Success      *SuccessExecutionResult `json:"success,omitempty"` //交易成功时的返回信息
	ErrorMessage *string                 `json:"error_message,omitempty"`
	Failure      *SuccessExecutionResult `json:"Failure,omitempty"` //交易失败时的繁华信息
}

type SuccessExecutionResult struct {
	Transfers    []string `json:"transfers"`
	Cost         string   `json:"cost"`
	ErrorMessage string   `json:"error_message"`
}

type storedValueResult struct {
	StoredValue StoredValue `json:"stored_value"`
}

type StoredValue struct {
	CLValue         *JsonCLValue          `json:"CLValue,omitempty"`
	Account         *JsonAccount          `json:"Account,omitempty"`
	Contract        *JsonContractMetadata `json:"Contract,omitempty"`
	ContractWASM    *string               `json:"ContractWASM,omitempty"`
	ContractPackage *string               `json:"ContractPackage,omitempty"`
	Transfer        *TransferResponse     `json:"Transfer,omitempty"`
	DeployInfo      *JsonDeployInfo       `json:"DeployInfo,omitempty"`
}

type JsonCLValue struct {
	Bytes  string      `json:"bytes"`
	CLType string      `json:"cl_type"`
	Parsed interface{} `json:"parsed"`
}

type JsonAccount struct {
	AccountHash      string           `json:"account_hash"`
	NamedKeys        []NamedKey       `json:"named_keys"`
	MainPurse        string           `json:"main_purse"`
	AssociatedKeys   []AssociatedKey  `json:"associated_keys"`
	ActionThresholds ActionThresholds `json:"action_thresholds"`
}

type NamedKey struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type AssociatedKey struct {
	AccountHash string `json:"account_hash"`
	Weight      uint64 `json:"weight"`
}

type ActionThresholds struct {
	Deployment    uint64 `json:"deployment"`
	KeyManagement uint64 `json:"key_management"`
}

type JsonContractMetadata struct {
	ContractPackageHash string `json:"contract_package_hash"`
	ContractWasmHash    string `json:"contract_wasm_hash"`
	ProtocolVersion     string `json:"protocol_version"`
}

type JsonDeployInfo struct {
	DeployHash string   `json:"deploy_hash"`
	Transfers  []string `json:"transfers"`
	From       string   `json:"from"`
	Source     string   `json:"source"`
	Gas        string   `json:"gas"`
}

type blockParams struct {
	BlockIdentifier blockIdentifier `json:"block_identifier"`
}

type blockIdentifier struct {
	Hash   string `json:"Hash,omitempty"`
	Height uint64 `json:"Height,omitempty"`
}

type balanceResponse struct {
	BalanceValue string `json:"balance_value"`
}

type ValidatorWeight struct {
	PublicKey string `json:"public_key"`
	Weight    string `json:"weight"`
}

type EraValidators struct {
	EraId            int               `json:"era_id"`
	ValidatorWeights []ValidatorWeight `json:"validator_weights"`
}

type AuctionState struct {
	StateRootHash string          `json:"state_root_hash"`
	BlockHeight   uint64          `json:"block_height"`
	EraValidators []EraValidators `json:"era_validators"`
	Bids          []BidInfo       `json:"bids"`
}

type ValidatorPesponse struct {
	Version      string       `json:"api_version"`
	AuctionState AuctionState `json:"auction_state"`
}

type StatusResult struct {
	LastAddedBlock BlockInfo `json:"last_added_block_info"`
	BuildVersion   string    `json:"build_version"`
	ApiVersion     string    `json:"api_version"`
	ChainspecName  string    `json:"chainspec_name"`
}

type Peer struct {
	NodeId  string `json:"node_id"`
	Address string `json:"address"`
}

type PeerResult struct {
	Peers []Peer `json:"peers"`
}

type StateRootHashResult struct {
	StateRootHash string `json:"state_root_hash"`
}

type Session struct {
	Transfer struct {
		Args [][]interface{} `json:"args"`
	} `json:"Transfer"`
}

type AccountInfo struct {
	ApiVersion string `json:"api_version"`
	Account    struct {
		AccountHash    string        `json:"account_hash"`
		NamedKeys      []interface{} `json:"named_keys"`
		MainPurse      string        `json:"main_purse"`
		AssociatedKeys []struct {
			AccountHash string `json:"account_hash"`
			Weight      int    `json:"weight"`
		} `json:"associated_keys"`
		ActionThresholds struct {
			Deployment    int `json:"deployment"`
			KeyManagement int `json:"key_management"`
		} `json:"action_thresholds"`
	} `json:"account"`
	MerkleProof string `json:"merkle_proof"`
}

type StackInfo struct {
	PublicKey    string        `json:"public_key"`
	StakedAmount bigint.BigInt `json:"staked_amount"`
	BondingPurse string        `json:"bonding_purse"`
	Delegatee    string        `json:"delegatee"`
}

type BidInfo struct {
	PublicKey string `json:"public_key"`
	Bid       struct {
		BondingPurse   string        `json:"bonding_purse"`
		StakedAmount   bigint.BigInt `json:"staked_amount"`
		DelegationRate int           `json:"delegation_rate"`
		Delegators     []StackInfo   `json:"delegators"`
		Inactive       bool          `json:"inactive"`
	} `json:"bid"`
}
