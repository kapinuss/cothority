package ch.epfl.dedis.integration;

import ch.epfl.dedis.byzgen.CalypsoFactory;
import com.github.dockerjava.api.DockerClient;
import com.github.dockerjava.api.command.ExecCreateCmdResponse;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.testcontainers.containers.Container;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.output.FrameConsumerResultCallback;
import org.testcontainers.containers.output.Slf4jLogConsumer;
import org.testcontainers.containers.wait.strategy.Wait;
import org.testcontainers.images.builder.ImageFromDockerfile;

import java.io.IOException;
import java.time.LocalDateTime;
import java.util.Arrays;
import java.util.List;

public class DockerTestServerController extends TestServerController {
    private static final Logger logger = LoggerFactory.getLogger(DockerTestServerController.class);
    private static final String TEST_SERVER_IMAGE_NAME = "dedis/conode-test:latest";
    private static final String TEMPORARY_DOCKER_IMAGE = "conode-test-run";

    private final GenericContainer<?> blockchainContainer;

    protected DockerTestServerController() {
        logger.warn("local docker will be started for tests.");
        logger.info("This test run assumes that image " + TEST_SERVER_IMAGE_NAME + " is available in your system.");
        logger.info("To build such image you should run `make docker docker_test` - such run will create base image and image with test keys.");
        logger.info("For a test run this code will create additional docker image with name " + TEMPORARY_DOCKER_IMAGE +
                ", at the end this additional image will be automatically deleted");
        try {
            blockchainContainer = new GenericContainer<>(
                    new ImageFromDockerfile(TEMPORARY_DOCKER_IMAGE, true)
                            .withDockerfileFromBuilder(builder -> builder
                                    .from(TEST_SERVER_IMAGE_NAME)
                                    .expose(7002, 7003, 7004, 7005, 7006, 7007, 7008, 7009, 7010, 7011, 7012, 7013, 7014, 7015))
            );

            blockchainContainer.setPortBindings(Arrays.asList(
                    "7002:7002", "7003:7003",
                    "7004:7004", "7005:7005",
                    "7006:7006", "7007:7007",
                    "7008:7008", "7009:7009",
                    "7010:7010", "7011:7011",
                    "7012:7012", "7013:7013",
                    "7014:7014", "7015:7015"));
            blockchainContainer.withExposedPorts(7002, 7003, 7004, 7005, 7006, 7007, 7008, 7009);
            blockchainContainer.withExtraHost("conode1", "127.0.0.1");
            blockchainContainer.withExtraHost("conode2", "127.0.0.1");
            blockchainContainer.withExtraHost("conode3", "127.0.0.1");
            blockchainContainer.withExtraHost("conode4", "127.0.0.1");
            blockchainContainer.withExtraHost("conode5", "127.0.0.1");
            blockchainContainer.withExtraHost("conode6", "127.0.0.1");
            blockchainContainer.withExtraHost("conode7", "127.0.0.1");
            blockchainContainer.waitingFor(Wait.forListeningPort());
            blockchainContainer.start();
            Slf4jLogConsumer logConsumer = new Slf4jLogConsumer(logger);
            blockchainContainer.withLogConsumer(logConsumer);
            blockchainContainer.followOutput(logConsumer);
            logger.info("Started at {}", LocalDateTime.now());
        } catch (Exception e) {
            logger.info("Exception at {}", LocalDateTime.now());
            throw new IllegalStateException("Cannot start docker image with test server. Please ensure that local conodes are not running.", e);
        }
    }

    @Override
    public void startConode(int nodeNumber) throws InterruptedException {
        if (nodeNumber <= 0) {
            throw new InterruptedException("Node numbering starts at 1!");
        }
        logger.info("Starting container co{}/private.toml", nodeNumber);
        runCmdInBackground(blockchainContainer, "env", "COTHORITY_ALLOW_INSECURE_ADMIN=1", "conode", "-d", "2", "-c", "co" + nodeNumber + "/private.toml", "server");
        // Wait a bit for the server to actually start.
        Thread.sleep(1000);
    }

    @Override
    public void killConode(int nodeNumber) throws IOException, InterruptedException {
        if (nodeNumber <= 0) {
            throw new InterruptedException("Node numbering starts at 1!");
        }
        logger.info("Killing container co{}/private.toml", nodeNumber);
        Container.ExecResult psResults = blockchainContainer.execInContainer("ps", "-o", "pid=,command=", "-C", "conode");
        for (String psLine : psResults.getStdout().split("\\n")) {
            if (psLine.contains("co" + nodeNumber + "/private.toml")) {
                String pid = psLine.trim().split("\\s")[0];
                blockchainContainer.execInContainer("kill", pid);
                break;
            }
        }
    }

    /**
     * We only get 4 conodes because the run_conode.sh file (from the Dockerfile) only starts 4 conodes.
     * The other conodes (5 to 7) are used for testing roster changes.
     */
    @Override
    public List<CalypsoFactory.ConodeAddress> getConodes() {
        return Arrays.asList(
                new CalypsoFactory.ConodeAddress(buildURI("tls://" + blockchainContainer.getContainerIpAddress() + ":7002"), CONODE_PUB_1),
                new CalypsoFactory.ConodeAddress(buildURI("tls://localhost:7004"), CONODE_PUB_2),
                new CalypsoFactory.ConodeAddress(buildURI("tls://localhost:7006"), CONODE_PUB_3),
                new CalypsoFactory.ConodeAddress(buildURI("tls://localhost:7008"), CONODE_PUB_4));
    }

    private void runCmdInBackground(GenericContainer container, String... cmd) throws InterruptedException {
        DockerClient dockerClient = container.getDockerClient();

        ExecCreateCmdResponse execCreateCmdResponse = dockerClient.execCreateCmd(container.getContainerId())
                .withAttachStdout(false)
                .withAttachStderr(false)
                .withAttachStdin(false)
                .withCmd(cmd)
                .exec();

        dockerClient.execStartCmd(execCreateCmdResponse.getId())
                .exec(new FrameConsumerResultCallback()).awaitStarted();
    }
}
