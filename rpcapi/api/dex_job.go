package api

import (
	"fmt"
	"github.com/vitelabs/go-vite/chain"
	"github.com/vitelabs/go-vite/common/types"
	"github.com/vitelabs/go-vite/log15"
	"github.com/vitelabs/go-vite/vite"
	"github.com/vitelabs/go-vite/vm/contracts/dex"
	"math/big"
)

type DexJobApi struct {
	vite  *vite.Vite
	chain chain.Chain
	log   log15.Logger
}

func NewDexJobApi(vite *vite.Vite) *DexJobApi {
	return &DexJobApi{
		vite:  vite,
		chain: vite.Chain(),
		log:   log15.New("module", "rpc_api/dexjob_api"),
	}
}

func (f DexJobApi) String() string {
	return "DexJobApi"
}

func (f DexJobApi) TriggerMine(periodId uint64) error {
	db, err := getDb(f.chain, types.AddressDexFund)
	if err != nil {
		return err
	}
	amount := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(10000))
	amtForMarkets := map[int32]*big.Int{dex.ViteTokenType : amount, dex.EthTokenType : amount, dex.BtcTokenType : amount, dex.UsdTokenType : amount}
	if refund, err := dex.DoMineVxForFee(db, getConsensusReader(f.vite), periodId, amtForMarkets); err != nil {
		return err
	} else {
		fmt.Printf("refund %s\n", refund.String())
	}
	return nil
}