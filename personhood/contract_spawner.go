package personhood

import (
	"crypto/sha256"
	"errors"

	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/contracts"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
)

// ContractSpawnerID denotes a contract that can spawn new instances.
var ContractSpawnerID = "spawner"

func contractSpawnerFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &contractSpawner{}
	err := protobuf.Decode(in, &c.SpawnerStruct)
	if err != nil {
		return nil, errors.New("couldn't unmarshal instance data: " + err.Error())
	}
	return c, nil
}

type contractSpawner struct {
	byzcoin.BasicContract
	SpawnerStruct
}

func (c contractSpawner) VerifyInstruction(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, ctxHash []byte) error {
	if inst.GetType() != byzcoin.SpawnType {
		if err := inst.Verify(rst, ctxHash); err != nil {
			return err
		}
	}
	return nil
}

func (c *contractSpawner) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// Spawn creates a new coin account as a separate instance.
	ca := inst.DeriveID("")
	var instBuf []byte
	cID := string(inst.Spawn.Args.Search("contractID"))
	if inst.Spawn.ContractID == ContractSpawnerID {
		cID = ContractSpawnerID
		c.ParseArgs(inst.Spawn.Args)
		instBuf, err = protobuf.Encode(&c.SpawnerStruct)
		if err != nil {
			return nil, nil, errors.New("couldn't encode SpawnerInstance: " + err.Error())
		}
	} else {
		switch cID {
		case byzcoin.ContractDarcID:
			if err = c.getCoins(cout, c.CostDarc); err != nil {
				return
			}
			instBuf = inst.Spawn.Args.Search("darc")
			d, err := darc.NewFromProtobuf(instBuf)
			if err != nil {
				return nil, nil, err
			}
			ca = byzcoin.NewInstanceID(d.GetBaseID())
		case contracts.ContractCoinID:
			if err = c.getCoins(cout, c.CostCoin); err != nil {
				return
			}
			coin := &byzcoin.Coin{
				Name: byzcoin.NewInstanceID(inst.Spawn.Args.Search("coinName")),
			}
			for i := range cout {
				if cout[i].Name.Equal(coin.Name) {
					coin.Value = cout[i].Value
					log.Lvl2("Adding initial balance:", coin.Value)
					cout[i].SafeSub(coin.Value)
				}
			}
			h := sha256.New()
			h.Write([]byte("coin"))
			h.Write(inst.Spawn.Args.Search("pubKey"))
			ca = byzcoin.NewInstanceID(h.Sum(nil))
			instBuf, err = protobuf.Encode(coin)
			if err != nil {
				return nil, nil, err
			}
		default:
			log.Print("Unknown contract", cID)
			return nil, nil, errors.New("don't know how to spawn this type of contract")
		}
	}
	log.Lvlf3("Spawning %s instance to %x", cID, ca.Slice())
	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, ca, cID, instBuf, darcID),
	}
	return
}

func (c contractSpawner) getCoins(coins []byzcoin.Coin, cost byzcoin.Coin) error {
	if cost.Value == 0 {
		return nil
	}
	for i := range coins {
		if coins[i].Name.Equal(cost.Name) {
			if coins[i].Value >= cost.Value {
				coins[i].SafeSub(cost.Value)
				return nil
			}
		}
	}
	return errors.New("don't have enough coins for spawning")
}

func (c *contractSpawner) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.Invoke.Command {
	case "transfer":
		// transfer sends a given amount of coins to another account.
		target := inst.Invoke.Args.Search("destination")
		var cid string
		_, _, cid, _, err = rst.GetValues(target)
		if err == nil && cid != ContractSpawnerID {
			err = errors.New("destination is not a coin contract")
		}
		if err != nil {
			return
		}

		// sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, byzcoin.NewInstanceID(target),
		// ContractSpawnerID, targetBuf, did))
	default:
		err = errors.New("Personhood contract can only")
		return
	}

	// Finally update the coin value.
	var ciBuf []byte
	ciBuf, err = protobuf.Encode(&c.SpawnerStruct)
	sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
		ContractSpawnerID, ciBuf, darcID))
	return
}

func (c *contractSpawner) Delete(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	sc = byzcoin.StateChanges{
		byzcoin.NewStateChange(byzcoin.Remove, inst.InstanceID, ContractSpawnerID, nil, darcID),
	}
	return
}

func (ss *SpawnerStruct) ParseArgs(args byzcoin.Arguments) error {
	for _, cost := range []struct {
		name string
		cost *byzcoin.Coin
	}{
		{"costDarc", &ss.CostDarc},
		{"costCoin", &ss.CostCoin},
		{"costCredential", &ss.CostCredential},
		{"costParty", &ss.CostParty},
	} {
		if arg := args.Search(cost.name); arg != nil {
			err := protobuf.Decode(arg, cost.cost)
			if err != nil {
				return err
			}
		} else {
			cost.cost = &byzcoin.Coin{contracts.CoinName, 100}
		}
	}
	return nil
}
