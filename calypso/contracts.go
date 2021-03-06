package calypso

import (
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// ContractWriteID references a write contract system-wide.
const ContractWriteID = "calypsoWrite"

type contractWr struct {
	byzcoin.BasicContract
	Write
}

func contractWriteFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &contractWr{}

	err := protobuf.DecodeWithConstructors(in, &c.Write, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, errors.New("couldn't unmarshal write: " + err.Error())
	}
	return c, nil
}

func (c *contractWr) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.Spawn.ContractID {
	case ContractWriteID:
		w := inst.Spawn.Args.Search("write")
		if w == nil || len(w) == 0 {
			err = errors.New("need a write request in 'write' argument")
			return
		}
		err = protobuf.DecodeWithConstructors(w, &c.Write, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			err = errors.New("couldn't unmarshal write: " + err.Error())
			return
		}
		if err = c.Write.CheckProof(cothority.Suite, darcID); err != nil {
			err = errors.New("proof of write failed: " + err.Error())
			return
		}
		instID := inst.DeriveID("")
		log.Lvlf3("Successfully verified write request and will store in %x", instID)
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, instID, ContractWriteID, w, darcID))
	case ContractReadID:
		var rd Read
		r := inst.Spawn.Args.Search("read")
		if r == nil || len(r) == 0 {
			return nil, nil, errors.New("need a read argument")
		}
		err = protobuf.DecodeWithConstructors(r, &rd, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return nil, nil, errors.New("passed read argument is invalid: " + err.Error())
		}
		_, _, wc, _, err := rst.GetValues(rd.Write.Slice())
		if err != nil {
			return nil, nil, errors.New("referenced write-id is not correct: " + err.Error())
		}
		if wc != ContractWriteID {
			return nil, nil, errors.New("referenced write-id is not a write instance, got " + wc)
		}
		sc = byzcoin.StateChanges{byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""), ContractReadID, r, darcID)}
	default:
		err = errors.New("can only spawn writes and reads")
	}
	return
}

// ContractReadID references a read contract system-wide.
const ContractReadID = "calypsoRead"

type contractRe struct {
	byzcoin.BasicContract
	Read
}

func contractReadFromBytes(in []byte) (byzcoin.Contract, error) {
	return nil, errors.New("calypso read instances are never instantiated")
}

// ContractLongTermSecretID is the contract ID for updating the LTS roster.
var ContractLongTermSecretID = "longTermSecret"

type contractLTS struct {
	byzcoin.BasicContract
	LtsInstanceInfo LtsInstanceInfo
}

func contractLTSFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &contractLTS{}

	err := protobuf.DecodeWithConstructors(in, &c.LtsInstanceInfo, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, errors.New("couldn't unmarshal LtsInfo: " + err.Error())
	}
	return c, nil
}

func (c *contractLTS) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) ([]byzcoin.StateChange, []byzcoin.Coin, error) {
	var darcID darc.ID
	_, _, _, darcID, err := rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	if inst.Spawn.ContractID != ContractLongTermSecretID {
		return nil, nil, errors.New("can only spawn long-term-secret instances")
	}
	infoBuf := inst.Spawn.Args.Search("lts_instance_info")
	if infoBuf == nil || len(infoBuf) == 0 {
		return nil, nil, errors.New("need a lts_instance_info argument")
	}
	var info LtsInstanceInfo
	err = protobuf.DecodeWithConstructors(infoBuf, &info, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, nil, errors.New("passed lts_instance_info argument is invalid: " + err.Error())
	}
	return byzcoin.StateChanges{byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""), ContractLongTermSecretID, infoBuf, darcID)}, coins, nil
}

func (c *contractLTS) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) ([]byzcoin.StateChange, []byzcoin.Coin, error) {
	var darcID darc.ID
	curBuf, _, _, darcID, err := rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	if inst.Invoke.Command != "reshare" {
		return nil, nil, errors.New("can only reshare long-term secrets")
	}
	infoBuf := inst.Invoke.Args.Search("lts_instance_info")
	if infoBuf == nil || len(infoBuf) == 0 {
		return nil, nil, errors.New("need a lts_instance_info argument")
	}

	var curInfo, newInfo LtsInstanceInfo
	err = protobuf.DecodeWithConstructors(infoBuf, &newInfo, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, nil, errors.New("passed lts_instance_info argument is invalid: " + err.Error())
	}
	err = protobuf.DecodeWithConstructors(curBuf, &curInfo, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, nil, errors.New("current info is invalid: " + err.Error())
	}

	// Verify the intersection between new roster and the old one. There must be
	// at least a threshold of nodes in the intersection.
	n := len(curInfo.Roster.List)
	overlap := intersectRosters(&curInfo.Roster, &newInfo.Roster)
	thr := n - (n-1)/3
	if overlap < thr {
		return nil, nil, errors.New("new roster does not overlap enough with current roster")
	}

	return byzcoin.StateChanges{byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID, ContractLongTermSecretID, infoBuf, darcID)}, coins, nil
}

func intersectRosters(r1, r2 *onet.Roster) int {
	res := 0
	for _, x := range r2.List {
		if i, _ := r1.Search(x.ID); i != -1 {
			res++
		}
	}
	return res
}
