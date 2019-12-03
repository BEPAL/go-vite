package api

import (
	"encoding/hex"
	"fmt"
	"github.com/pkg/errors"
	"github.com/vitelabs/go-vite/chain"
	"github.com/vitelabs/go-vite/common/types"
	"github.com/vitelabs/go-vite/log15"
	apidex "github.com/vitelabs/go-vite/rpcapi/api/dex"
	"github.com/vitelabs/go-vite/vite"
	"github.com/vitelabs/go-vite/vm/contracts/dex"
	"math/big"
)

type DexApi struct {
	vite  *vite.Vite
	chain chain.Chain
	log   log15.Logger
}

func NewDexApi(vite *vite.Vite) *DexApi {
	return &DexApi{
		vite:  vite,
		chain: vite.Chain(),
		log:   log15.New("module", "rpc_api/dex_api"),
	}
}

func (f DexApi) String() string {
	return "DexApi"
}

type AccountBalanceInfo struct {
	TokenInfo *RpcTokenInfo `json:"tokenInfo,omitempty"`
	Available string        `json:"available"`
	Locked    string        `json:"locked"`
}

func (f DexApi) GetAccountBalanceInfo(addr types.Address, tokenId *types.TokenTypeId) (map[types.TokenTypeId]*AccountBalanceInfo, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	fund, _ := dex.GetFund(db, addr)
	accounts, err := dex.GetAccounts(fund, tokenId)
	if err != nil {
		return nil, err
	}

	balanceInfo := make(map[types.TokenTypeId]*AccountBalanceInfo, 0)
	for _, v := range accounts {
		tokenInfo, err := f.chain.GetTokenInfoById(v.Token)
		if err != nil {
			return nil, err
		}
		info := &AccountBalanceInfo{TokenInfo: RawTokenInfoToRpc(tokenInfo, v.Token)}
		a := "0"
		if v.Available != nil {
			a = v.Available.String()
		}
		info.Available = a

		l := "0"
		if v.Locked != nil {
			l = v.Locked.String()
		}
		info.Locked = l
		balanceInfo[v.Token] = info
	}
	return balanceInfo, nil
}

func (f DexApi) GetAccountBalanceInfoByStatus(addr types.Address, tokenId *types.TokenTypeId, status byte) (map[types.TokenTypeId]string, error) {
	if status != 0 && status != 1 && status != 2 {
		return nil, errors.New("args's status error, 1 for available, 2 for locked, 0 for total")
	}

	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	fund, _ := dex.GetFund(db, addr)
	fundInfo, err := dex.GetAccounts(fund, tokenId)
	if err != nil {
		return nil, err
	}

	balanceInfo := make(map[types.TokenTypeId]string, 0)
	for _, v := range fundInfo {
		amount := big.NewInt(0)
		if a := v.Available; a != nil {
			if status == 0 || status == 1 {
				amount.Add(amount, a)
			}
		}
		if l := v.Locked; l != nil {
			if status == 0 || status == 2 {
				amount.Add(amount, l)
			}
		}
		balanceInfo[v.Token] = amount.String()
	}
	return balanceInfo, nil
}

func (f DexApi) GetTokenInfo(token types.TokenTypeId) (*apidex.RpcDexTokenInfo, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if tokenInfo, ok := dex.GetTokenInfo(db, token); ok {
		return apidex.TokenInfoToRpc(tokenInfo, token), nil
	} else {
		return nil, dex.InvalidTokenErr
	}
}

func (f DexApi) GetMarketInfo(tradeToken, quoteToken types.TokenTypeId) (*apidex.NewRpcMarketInfo, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if marketInfo, ok := dex.GetMarketInfo(db, tradeToken, quoteToken); ok {
		return apidex.MarketInfoToNewRpc(marketInfo), nil
	} else {
		return nil, dex.TradeMarketNotExistsErr
	}
}

func (f DexApi) GetDividendPoolsInfo() (map[types.TokenTypeId]*apidex.DividendPoolInfo, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	var pools map[types.TokenTypeId]*apidex.DividendPoolInfo
	if dexFeesByPeriod, ok := dex.GetCurrentDexFees(db, apidex.GetConsensusReader(f.vite)); !ok {
		return nil, nil
	} else {
		pools = make(map[types.TokenTypeId]*apidex.DividendPoolInfo)
		for _, pool := range dexFeesByPeriod.FeesForDividend {
			tk, _ := types.BytesToTokenTypeId(pool.Token)
			if tokenInfo, ok := dex.GetTokenInfo(db, tk); !ok {
				return nil, dex.InvalidTokenErr
			} else {
				amt := new(big.Int).SetBytes(pool.DividendPoolAmount)
				pool := &apidex.DividendPoolInfo{amt.String(), tokenInfo.QuoteTokenType, apidex.TokenInfoToRpc(tokenInfo, tk)}
				pools[tk] = pool
			}
		}
	}
	return pools, nil
}

func (f DexApi) HasStakedForVIP(address types.Address) (bool, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return false, err
	}
	_, ok := dex.GetVIPStaking(db, address)
	return ok, nil
}

func (f DexApi) HasStakedForSVIP(address types.Address) (bool, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return false, err
	}
	_, ok := dex.GetSuperVIPStaking(db, address)
	return ok, nil
}

func (f DexApi) IsDexStopped() (bool, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return false, err
	}
	return dex.IsDexStopped(db), nil
}

func (f DexApi) GetInviteCode(address types.Address) (uint32, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return 0, err
	}
	return dex.GetCodeByInviter(db, address), nil
}

func (f DexApi) GetInviteCodeBinding(address types.Address) (uint32, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return 0, err
	}
	if inviter, err := dex.GetInviterByInvitee(db, address); err != nil {
		if err == dex.NotBindInviterErr {
			return 0, nil
		} else {
			return 0, err
		}
	} else {
		return dex.GetCodeByInviter(db, *inviter), nil
	}
}

func (f DexApi) IsInviteCodeValid(code uint32) (bool, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return false, err
	}
	if addr, err := dex.GetInviterByCode(db, code); err != nil && err != dex.InvalidInviteCodeErr {
		return false, err
	} else {
		return addr != nil, nil
	}
}

func (f DexApi) IsMarketDelegatedTo(principal, agent types.Address, tradeToken, quoteToken types.TokenTypeId) (bool, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return false, err
	}
	if marketInfo, ok := dex.GetMarketInfo(db, tradeToken, quoteToken); !ok {
		return false, dex.TradeMarketNotExistsErr
	} else {
		return dex.IsMarketGrantedToAgent(db, principal, agent, marketInfo.MarketId), nil
	}
}

func (f DexApi) GetCurrentMiningInfo() (mineInfo *apidex.NewRpcVxMineInfo, err error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	periodId := dex.GetCurrentPeriodId(db, apidex.GetConsensusReader(f.vite))
	toMine := dex.GetVxToMineByPeriodId(db, periodId)
	available := dex.GetVxMinePool(db)
	if toMine.Cmp(available) > 0 {
		toMine = available
	}
	mineInfo = new(apidex.NewRpcVxMineInfo)
	if toMine.Sign() == 0 {
		err = fmt.Errorf("no vx available on mine")
		return
	}
	mineInfo.HistoryMinedSum = new(big.Int).Sub(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(100000000)), available).String()
	mineInfo.Total = toMine.String()
	var (
		amountForItems map[int32]*big.Int
		amount         *big.Int
		success        bool
	)
	if amountForItems, available, success = dex.GetVxAmountsForEqualItems(db, periodId, available, dex.RateSumForFeeMine, dex.ViteTokenType, dex.UsdTokenType); success {
		mineInfo.FeeMineDetail = make(map[int32]string)
		feeMineSum := new(big.Int)
		for tokenType, amount := range amountForItems {
			mineInfo.FeeMineDetail[tokenType] = amount.String()
			feeMineSum.Add(feeMineSum, amount)
		}
		mineInfo.FeeMineTotal = feeMineSum.String()
	} else {
		return
	}
	if amount, available, success = dex.GetVxAmountToMine(db, periodId, available, dex.RateForStakingMine); success {
		mineInfo.StakingMine = amount.String()
	} else {
		return
	}
	if amountForItems, available, success = dex.GetVxAmountsForEqualItems(db, periodId, available, dex.RateSumForMakerAndMaintainerMine, dex.MineForMaker, dex.MineForMaintainer); success {
		mineInfo.MakerMine = amountForItems[dex.MineForMaker].String()
	}
	return
}

func (f DexApi) GetCurrentFeesValidForMining() (fees map[int32]string, err error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if dexFeesByPeriod, ok := dex.GetCurrentDexFees(db, apidex.GetConsensusReader(f.vite)); !ok {
		return
	} else {
		fees = make(map[int32]string, len(dexFeesByPeriod.FeesForMine))
		for _, feeForMine := range dexFeesByPeriod.FeesForMine {
			fees[feeForMine.QuoteTokenType] = new(big.Int).Add(new(big.Int).SetBytes(feeForMine.BaseAmount), new(big.Int).SetBytes(feeForMine.InviteBonusAmount)).String()
		}
		return
	}
}

func (f DexApi) GetCurrentStakingValidForMining() (string, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return "0", err
	}
	if miningStakings, ok := dex.GetDexMiningStakings(db); !ok {
		return "0", nil
	} else {
		stakingsLen := len(miningStakings.Stakings)
		if stakingsLen == 0 {
			return "0", nil
		} else {
			return new(big.Int).SetBytes(miningStakings.Stakings[stakingsLen-1].Amount).String(), nil
		}
	}
}

func (f DexApi) GetOrderById(orderIdStr string) (*apidex.RpcOrder, error) {
	orderId, err := hex.DecodeString(orderIdStr)
	if err != nil {
		return nil, err
	}
	if db, err := getVmDb(f.chain, types.AddressDexTrade); err != nil {
		return nil, err
	} else {
		return apidex.InnerGetOrderById(db, orderId)
	}
}

func (f DexApi) GetOrderByTransactionHash(sendHash types.Hash) (*apidex.RpcOrder, error) {
	if db, err := getVmDb(f.chain, types.AddressDexTrade); err != nil {
		return nil, err
	} else {
		if orderId, ok := dex.GetOrderIdByHash(db, sendHash.Bytes()); !ok {
			return nil, dex.OrderNotExistsErr
		} else {
			return apidex.InnerGetOrderById(db, orderId)
		}
	}
}

func (f DexApi) GetOrdersForMarket(tradeToken, quoteToken types.TokenTypeId, side bool, begin, end int) (ordersRes *apidex.OrdersRes, err error) {
	if fundDb, err := getVmDb(f.chain, types.AddressDexFund); err != nil {
		return nil, err
	} else {
		if marketInfo, ok := dex.GetMarketInfo(fundDb, tradeToken, quoteToken); !ok {
			return nil, dex.TradeMarketNotExistsErr
		} else {
			if tradeDb, err := getVmDb(f.chain, types.AddressDexTrade); err != nil {
				return nil, err
			} else {
				matcher := dex.NewMatcherWithMarketInfo(tradeDb, marketInfo)
				if ods, size, err := matcher.GetOrdersFromMarket(side, begin, end); err == nil {
					ordersRes = &apidex.OrdersRes{apidex.OrdersToRpc(ods), size}
					return ordersRes, err
				} else {
					return &apidex.OrdersRes{apidex.OrdersToRpc(ods), size}, err
				}
			}
		}
	}
}

type DexPrivateApi struct {
	vite  *vite.Vite
	chain chain.Chain
	log   log15.Logger
}

func NewDexPrivateApi(vite *vite.Vite) *DexPrivateApi {
	return &DexPrivateApi{
		vite:  vite,
		chain: vite.Chain(),
		log:   log15.New("module", "rpc_api/dex_private_api"),
	}
}

func (f DexPrivateApi) String() string {
	return "DexPrivateApi"
}

func (f DexPrivateApi) GetOwner() (*types.Address, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	return dex.GetOwner(db)
}

func (f DexPrivateApi) GetTime() (int64, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return -1, err
	}
	return dex.GetTimestampInt64(db), nil
}

func (f DexPrivateApi) GetPeriodId() (uint64, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return 0, err
	}
	return dex.GetCurrentPeriodId(db, apidex.GetConsensusReader(f.vite)), nil
}

func (f DexPrivateApi) GetCurrentDexFees() (*apidex.RpcDexFeesByPeriod, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if dexFeesByPeriod, ok := dex.GetCurrentDexFees(db, apidex.GetConsensusReader(f.vite)); !ok {
		return nil, nil
	} else {
		return apidex.DexFeesByPeriodToRpc(dexFeesByPeriod), nil
	}
}

func (f DexPrivateApi) GetDexFeesByPeriod(periodId uint64) (*apidex.RpcDexFeesByPeriod, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if dexFeesByPeriod, ok := dex.GetDexFeesByPeriodId(db, periodId); !ok {
		return nil, nil
	} else {
		return apidex.DexFeesByPeriodToRpc(dexFeesByPeriod), nil
	}
}

func (f DexPrivateApi) GetCurrentOperatorFees(operator types.Address) (*apidex.RpcOperatorFeesByPeriod, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if operatorFeesByPeriod, ok := dex.GetCurrentOperatorFees(db, apidex.GetConsensusReader(f.vite), operator.Bytes()); !ok {
		return nil, nil
	} else {
		return apidex.OperatorFeesByPeriodToRpc(operatorFeesByPeriod), nil
	}
}

func (f DexPrivateApi) GetOperatorFeesByPeriod(periodId uint64, operator types.Address) (*apidex.RpcOperatorFeesByPeriod, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if operatorFeesByPeriod, ok := dex.GetOperatorFeesByPeriodId(db, operator.Bytes(), periodId); !ok {
		return nil, nil
	} else {
		return apidex.OperatorFeesByPeriodToRpc(operatorFeesByPeriod), nil
	}
}

func (f DexPrivateApi) GetAllFeesOfAddress(address types.Address) (*apidex.RpcUserFees, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if userFees, ok := dex.GetUserFees(db, address.Bytes()); !ok {
		return nil, nil
	} else {
		return apidex.UserFeesToRpc(userFees), nil
	}
}

func (f DexPrivateApi) GetAllTotalVxBalance() (*apidex.RpcVxFunds, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if vxSumFunds, ok := dex.GetVxSumFunds(db); !ok {
		return nil, nil
	} else {
		return apidex.VxFundsToRpc(vxSumFunds), nil
	}
}

func (f DexPrivateApi) GetAllVxBalanceByAddress(address types.Address) (*apidex.RpcVxFunds, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if vxFunds, ok := dex.GetVxFunds(db, address.Bytes()); !ok {
		return nil, nil
	} else {
		return apidex.VxFundsToRpc(vxFunds), nil
	}
}

func (f DexPrivateApi) GetVxPoolBalance() (string, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return "", err
	}
	balance := dex.GetVxMinePool(db)
	return balance.String(), nil
}

func (f DexPrivateApi) GetVIPStakingInfoByAddress(address types.Address) (*dex.VIPStaking, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if info, ok := dex.GetVIPStaking(db, address); ok {
		return info, nil
	} else {
		return nil, nil
	}
}

func (f DexPrivateApi) GetCurrentMiningStakingAmountByAddress(address types.Address) (string, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return "", err
	}
	return dex.GetMiningStakedAmount(db, address).String(), nil
}

func (f DexPrivateApi) GetAllMiningStakingInfoByAddress(address types.Address) (*apidex.RpcMiningStakings, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if miningStakings, ok := dex.GetMiningStakings(db, address); ok {
		return apidex.MiningStakingsToRpc(miningStakings), nil
	} else {
		return nil, nil
	}
}

func (f DexPrivateApi) GetAllMiningStakingInfo() (*apidex.RpcMiningStakings, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	if miningStakings, ok := dex.GetDexMiningStakings(db); ok {
		return apidex.MiningStakingsToRpc(miningStakings), nil
	} else {
		return nil, nil
	}
}

func (f DexPrivateApi) GetDexConfig() (map[string]string, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	configs := make(map[string]string)
	owner, _ := dex.GetOwner(db)
	configs["owner"] = owner.String()
	if timer := dex.GetTimeOracle(db); timer != nil {
		configs["timer"] = timer.String()
	}
	if trigger := dex.GetPeriodJobTrigger(db); trigger != nil {
		configs["trigger"] = trigger.String()
	}
	if mineProxy := dex.GetMakerMiningAdmin(db); mineProxy != nil {
		configs["mineProxy"] = mineProxy.String()
	}
	if maintainer := dex.GetMaintainer(db); maintainer != nil {
		configs["maintainer"] = maintainer.String()
	}
	return configs, nil
}

func (f DexPrivateApi) GetMinThresholdForTradeAndMining() (map[int]*apidex.RpcThresholdForTradeAndMine, error) {

	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	thresholds := make(map[int]*apidex.RpcThresholdForTradeAndMine, 4)
	for tokenType := dex.ViteTokenType; tokenType <= dex.UsdTokenType; tokenType++ {
		tradeThreshold := dex.GetTradeThreshold(db, int32(tokenType))
		mineThreshold := dex.GetMineThreshold(db, int32(tokenType))
		thresholds[tokenType] = &apidex.RpcThresholdForTradeAndMine{TradeThreshold: tradeThreshold.String(), MineThreshold: mineThreshold.String()}
	}
	return thresholds, nil
}

func (f DexPrivateApi) GetMakerMiningPool(periodId uint64) (string, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return "", err
	}
	balance := dex.GetMakerMiningPoolByPeriodId(db, periodId)
	return balance.String(), nil
}

func (f DexPrivateApi) GetLastPeriodIdByJobType(bizType uint8) (uint64, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return 0, err
	}
	if lastPeriodId := dex.GetLastJobPeriodIdByBizType(db, bizType); err != nil {
		return 0, err
	} else {
		return lastPeriodId, nil
	}
}

func (f DexPrivateApi) VerifyDexBalance() (*dex.FundVerifyRes, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	return dex.VerifyDexFundBalance(db, apidex.GetConsensusReader(f.vite)), nil
}

func (f DexPrivateApi) GetFirstMiningPeriodId() (uint64, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return 0, err
	}
	if firstPeriodId := dex.GetFirstMinedVxPeriodId(db); err != nil {
		return 0, err
	} else {
		return firstPeriodId, nil
	}
}

func (f DexPrivateApi) GetLastSettledMakerMiningInfo() (map[string]uint64, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return nil, err
	}
	lastSettleInfo := make(map[string]uint64)
	lastSettleInfo["period"] = dex.GetLastSettledMakerMinedVxPeriod(db)
	lastSettleInfo["page"] = uint64(dex.GetLastSettledMakerMinedVxPage(db))
	return lastSettleInfo, nil
}

func (f DexPrivateApi) IsNormalMiningStarted() (bool, error) {
	db, err := getVmDb(f.chain, types.AddressDexFund)
	if err != nil {
		return false, err
	}
	return dex.IsNormalMineStarted(db), nil
}

func (f DexPrivateApi) GetMarketInfoById(marketId int32) (ordersRes *apidex.RpcMarketInfo, err error) {
	if tradeDb, err := getVmDb(f.chain, types.AddressDexTrade); err != nil {
		return nil, err
	} else {
		if marketInfo, ok := dex.GetMarketInfoById(tradeDb, marketId); ok {
			return apidex.MarketInfoToRpc(marketInfo), nil
		} else {
			return nil, nil
		}
	}
}

func (f DexPrivateApi) GetTradeTimestamp() (timestamp int64, err error) {
	if tradeDb, err := getVmDb(f.chain, types.AddressDexTrade); err != nil {
		return -1, err
	} else {
		return dex.GetTradeTimestamp(tradeDb), nil
	}
}
