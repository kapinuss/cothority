package ch.epfl.dedis.byzcoin.contracts;

import ch.epfl.dedis.byzcoin.ByzCoinRPC;
import ch.epfl.dedis.byzcoin.Instance;
import ch.epfl.dedis.byzcoin.InstanceId;
import ch.epfl.dedis.byzcoin.Proof;
import ch.epfl.dedis.byzcoin.transaction.ClientTransaction;
import ch.epfl.dedis.byzcoin.transaction.Instruction;
import ch.epfl.dedis.byzcoin.transaction.Invoke;
import ch.epfl.dedis.lib.darc.Signer;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;
import java.util.List;

/**
 * ChainConfigInstance represents the only Configuration present in a ByzCoin ledger. This should not be used directly,
 * as ruining your chain config might have bad consequences for the liveness of the ByzCoin ledger. Do use
 * the corresponding methods in ByzCoinRPC.
 */
public class ChainConfigInstance {
    public static String ContractId = "config";
    private Instance instance;
    private ByzCoinRPC bc;
    private ChainConfigData chainConfig;

    private final static Logger logger = LoggerFactory.getLogger(ChainConfigInstance.class);

    /**
     * Instantiates the ChainConfig instance from the ByzCoin ledger.
     *
     * @param bc       a running ByzCoin service
     * @param instance a value instance
     */
    private ChainConfigInstance(ByzCoinRPC bc, Instance instance) throws CothorityNotFoundException {
        if (!instance.getContractId().equals(ContractId)) {
            logger.error("wrong contractId: {}", instance.getContractId());
            throw new CothorityNotFoundException("this is not a value instance");
        }
        this.bc = bc;
        this.instance = instance;
        this.chainConfig = new ChainConfigData(instance);
    }

    /**
     * Updates the value by getting the latest instance and updating it.
     *
     * @throws CothorityNotFoundException      if the chainConfiguration couldn't be found in ByzCoin
     * @throws CothorityCommunicationException if there was an communication error
     */
    public void update() throws CothorityCommunicationException, CothorityNotFoundException {
        instance = Instance.fromByzcoin(bc, instance.getId());
        chainConfig = new ChainConfigData(instance);
    }

    /**
     * Creates an instruction to evolve the value in byzcoin.
     *
     * @param newConfig the new config to store in the ChainConfig
     * @param ownerCtrs is the list of monotonically increasing counters that will go into the instruction,
     *                  they must match the signers who will eventually sign the instruction
     * @return Instruction to be sent to byzcoin
     * @throws CothorityCryptoException if there's a problem with the cryptography
     */
    public Instruction evolveChainConfigInstruction(ChainConfigData newConfig, List<Long> ownerCtrs) {
        Invoke inv = new Invoke("update_config", ContractId, newConfig.toProto().toByteArray());
        return new Instruction(instance.getId(), ownerCtrs, inv);
    }

    /**
     * Sends the instruction to change the Chain Config and returns immediately.
     * All the owners must have its identity in the current
     * darc as "invoke:update" rule.
     *
     * @param newConfig the new config to store
     * @param owners    a list of owners needed to evolve the configuration
     * @param ownerCtrs a list of counters which must map to the list of owners
     * @throws CothorityException if something goes wrong
     */
    public void evolveChainConfig(ChainConfigData newConfig, List<Signer> owners, List<Long> ownerCtrs) throws CothorityException {
        Instruction inst = evolveChainConfigInstruction(newConfig, ownerCtrs);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ct.signWith(owners);
        bc.sendTransaction(ct);
    }

    /**
     * Send the instruction to change the Chain Config and wait for it to be included. If you udpate the roster,
     * be sure to tell ByzCoinRPC that it has a new roster.
     *
     * @param newConfig the new config to sture
     * @param owners    a list of owners needed to evolve the configuration
     * @param ownerCtrs a list of counters which must map to the list of owners
     * @param wait      how many blocks to wait for inclusion of the instruction
     * @throws CothorityException if something goes wrong
     */
    public void evolveConfigAndWait(ChainConfigData newConfig, List<Signer> owners, List<Long> ownerCtrs, int wait) throws CothorityException {
        Instruction inst = evolveChainConfigInstruction(newConfig, ownerCtrs);
        ClientTransaction ct = new ClientTransaction(Arrays.asList(inst));
        ct.signWith(owners);
        bc.sendTransactionAndWait(ct, wait);
        chainConfig = newConfig;
    }

    /**
     * @return the id of the instance - should be the all null id.
     */
    public InstanceId getId() {
        return instance.getId();
    }

    /**
     * @return a copy of the configuration stored in this instance
     */
    public ChainConfigData getChainConfig() {
        return new ChainConfigData(chainConfig);
    }

    /**
     * @return the instance used.
     */
    public Instance getInstance() {
        return instance;
    }

    /**
     * Instantiates a new ChainConfigInstance given a working byzcoin service. As the
     * ChainConfig is always at id 0x00, there is no need for an instanceId.
     *
     * @param bc is a running ByzCoin service
     * @return the new ValueInstance
     * @throws CothorityNotFoundException      if the configuration is not where it is supposed to be
     * @throws CothorityCommunicationException if the communication throws an error
     */
    public static ChainConfigInstance fromByzcoin(ByzCoinRPC bc) throws CothorityNotFoundException, CothorityCommunicationException {
        return new ChainConfigInstance(bc, Instance.fromByzcoin(bc, new InstanceId(new byte[32])));
    }

    /**
     * Convenience function to connect to an existing ValueInstance.
     *
     * @param bc a running ByzCoin service
     * @param p  the proof for the valueInstance
     * @return the new ValueInstance
     * @throws CothorityNotFoundException      if the configuration is not where it is supposed to be
     * @throws CothorityCommunicationException if the communication throws an error
     */
    public static ChainConfigInstance fromByzcoin(ByzCoinRPC bc, Proof p) throws CothorityNotFoundException, CothorityCommunicationException {
        return new ChainConfigInstance(bc, Instance.fromProof(p));
    }
}
