package dex

import (
	"fmt"
	"github.com/vitelabs/go-vite/common/types"
	"github.com/vitelabs/go-vite/vm_db"
	"math/big"
)

//Note: allow dividend from specify periodId, former periods will be divided at that period
func DoFeesDividend(db vm_db.VmDb, periodId uint64) error {
	var (
		dexFeesByPeriodMap map[uint64]*DexFeesByPeriod
		vxSumFunds         *VxFunds
		err                error
		ok                 bool
	)

	//allow divide history fees that not divided yet
	if dexFeesByPeriodMap = GetNotFinishDividendDexFeesByPeriodMap(db, periodId); len(dexFeesByPeriodMap) == 0 { // no fee to divide
		return nil
	}
	if vxSumFunds, ok = GetVxSumFunds(db); !ok {
		return nil
	}
	foundVxSumFunds, vxSumAmtBytes, needUpdateVxSum, _ := MatchVxFundsByPeriod(vxSumFunds, periodId, false)
	//fmt.Printf("foundVxSumFunds %v, vxSumAmtBytes %s, needUpdateVxSum %v with periodId %d\n", foundVxSumFunds, new(big.Int).SetBytes(vxSumAmtBytes).String(), needUpdateVxSum, periodId)
	if !foundVxSumFunds { // not found vxSumFunds
		return nil
	}
	if needUpdateVxSum {
		SaveVxSumFunds(db, vxSumFunds)
	}
	vxSumAmt := new(big.Int).SetBytes(vxSumAmtBytes)
	if vxSumAmt.Sign() <= 0 {
		return nil
	}
	// sum fees from multi period not divided
	feeSumMap := make(map[types.TokenTypeId]*big.Int)
	for pId, fee := range dexFeesByPeriodMap {
		for _, feeAccount := range fee.FeesForDividend {
			if tokenId, err := types.BytesToTokenTypeId(feeAccount.Token); err != nil {
				return err
			} else {
				toDividendAmt, _ := splitDividendPool(feeAccount)
				if amt, ok := feeSumMap[tokenId]; !ok {
					feeSumMap[tokenId] = toDividendAmt
				} else {
					feeSumMap[tokenId] = amt.Add(amt, toDividendAmt)
				}
			}
		}
		MarkDexFeesFinishDividend(db, fee, pId)
	}

	var (
		userVxFundsKey, userVxFundsBytes []byte
	)

	iterator, err := db.NewStorageIterator(VxFundKeyPrefix)
	if err != nil {
		panic(err)
	}
	defer iterator.Release()

	feeSumWithTokens := MapToAmountWithTokens(feeSumMap)

	feeSumLeavedMap := make(map[types.TokenTypeId]*big.Int)
	dividedVxAmtMap := make(map[types.TokenTypeId]*big.Int)
	for {
		if len(feeSumMap) == 0 {
			break
		}
		if !iterator.Next() {
			if iterator.Error() != nil {
				panic(iterator.Error())
			}
			break
		}

		userVxFundsKey = iterator.Key()
		userVxFundsBytes = iterator.Value()
		if len(userVxFundsBytes) == 0 {
			continue
		}

		addressBytes := userVxFundsKey[len(VxFundKeyPrefix):]
		address := types.Address{}
		if err = address.SetBytes(addressBytes); err != nil {
			return err
		}
		userVxFunds := &VxFunds{}
		if err = userVxFunds.DeSerialize(userVxFundsBytes); err != nil {
			return err
		}

		var userFeeDividend = make(map[types.TokenTypeId]*big.Int)
		foundVxFunds, userVxAmtBytes, needUpdateVxFunds, needDeleteVxFunds := MatchVxFundsByPeriod(userVxFunds, periodId, true)
		if !foundVxFunds {
			continue
		}
		if needDeleteVxFunds {
			DeleteVxFunds(db, address.Bytes())
		} else if needUpdateVxFunds {
			SaveVxFunds(db, address.Bytes(), userVxFunds)
		}
		userVxAmount := new(big.Int).SetBytes(userVxAmtBytes)
		//fmt.Printf("address %s, userVxAmount %s, needDeleteVxFunds %v\n", string(address.Bytes()), userVxAmount.String(), needDeleteVxFunds)
		if !IsValidVxAmountForDividend(userVxAmount) { //skip vxAmount not valid for dividend
			continue
		}

		var finished bool
		for _, feeSumWtTk := range feeSumWithTokens {
			if feeSumWtTk.Deleted {
				continue
			}
			if _, ok = feeSumLeavedMap[feeSumWtTk.Token]; !ok {
				feeSumLeavedMap[feeSumWtTk.Token] = new(big.Int).Set(feeSumWtTk.Amount)
				dividedVxAmtMap[feeSumWtTk.Token] = big.NewInt(0)
			}
			//fmt.Printf("tokenId %s, address %s, vxSumAmt %s, userVxAmount %s, dividedVxAmt %s, toDivideFeeAmt %s, toDivideLeaveAmt %s\n", tokenId.String(), address.String(), vxSumAmt.String(), userVxAmount.String(), dividedVxAmtMap[tokenId], toDivideFeeAmt.String(), toDivideLeaveAmt.String())
			userFeeDividend[feeSumWtTk.Token], finished = DivideByProportion(vxSumAmt, userVxAmount, dividedVxAmtMap[feeSumWtTk.Token], feeSumWtTk.Amount, feeSumLeavedMap[feeSumWtTk.Token])
			if finished {
				feeSumWtTk.Deleted = true
				delete(feeSumMap, feeSumWtTk.Token)
			}
			AddFeeDividendEvent(db, address, feeSumWtTk.Token, userVxAmount, userFeeDividend[feeSumWtTk.Token])
		}
		if err = UpdateFund(db, address, userFeeDividend); err != nil {
			return err
		}
	}
	return err
}

func DoOperatorFeesDividend(db vm_db.VmDb, periodId uint64) error {
	iterator, err := db.NewStorageIterator(append(operatorFeesKeyPrefix, Uint64ToBytes(periodId)...))
	if err != nil {
		panic(err)
	}
	defer iterator.Release()
	for {
		var operatorFeesKey, operatorFeesBytes []byte
		if !iterator.Next() {
			if iterator.Error() != nil {
				panic(iterator.Error())
			}
			break
		}
		operatorFeesKey = iterator.Key() //3+8+21
		operatorFeesBytes = iterator.Value()
		if len(operatorFeesBytes) == 0 {
			continue
		}
		if len(operatorFeesKey) != 32 {
			panic(fmt.Errorf("invalid opearator fees key type"))
		}
		DeleteOperatorFeesByKey(db, operatorFeesKey)
		operatorFeesByPeriod := &OperatorFeesByPeriod{}
		if err = operatorFeesByPeriod.DeSerialize(operatorFeesBytes); err != nil {
			panic(err)
		}
		addr, err := types.BytesToAddress(operatorFeesKey[11:])
		if err != nil {
			panic(err)
		}
		userFund := make(map[types.TokenTypeId]*big.Int)
		for _, feeAcc := range operatorFeesByPeriod.OperatorFees {
			tokenId, err := types.BytesToTokenTypeId(feeAcc.Token)
			if err != nil {
				panic(err)
			}
			for _, mkFee := range feeAcc.MarketFees {
				if fd, hasToken := userFund[tokenId]; hasToken {
					userFund[tokenId] = new(big.Int).Add(fd, new(big.Int).SetBytes(mkFee.Amount))
				} else {
					userFund[tokenId] = new(big.Int).SetBytes(mkFee.Amount)
				}
				AddOperatorFeeDividendEvent(db, addr, mkFee)
			}
		}
		UpdateFund(db, addr, userFund)
	}
	return nil
}

func DivideByProportion(totalReferAmt, partReferAmt, dividedReferAmt, toDivideTotalAmt, toDivideLeaveAmt *big.Int) (proportionAmt *big.Int, finished bool) {
	dividedReferAmt.Add(dividedReferAmt, partReferAmt)
	proportion := new(big.Float).SetPrec(bigFloatPrec).Quo(new(big.Float).SetPrec(bigFloatPrec).SetInt(partReferAmt), new(big.Float).SetPrec(bigFloatPrec).SetInt(totalReferAmt))
	proportionAmt = RoundAmount(new(big.Float).SetPrec(bigFloatPrec).Mul(new(big.Float).SetPrec(bigFloatPrec).SetInt(toDivideTotalAmt), proportion))
	toDivideLeaveNewAmt := new(big.Int).Sub(toDivideLeaveAmt, proportionAmt)
	if toDivideLeaveNewAmt.Sign() <= 0 || dividedReferAmt.Cmp(totalReferAmt) >= 0 {
		proportionAmt.Set(toDivideLeaveAmt)
		finished = true
		toDivideLeaveAmt.SetInt64(0)
	} else {
		toDivideLeaveAmt.Set(toDivideLeaveNewAmt)
	}
	return proportionAmt, finished
}
