import {
  Center,
  HStack,
  VStack,
  Box,
  Image,
  Alert,
  AlertIcon,
  Divider,
  Heading,
  Stack,
  Text,
  FormControl,
  FormLabel,
  SimpleGrid,
  Switch,
  Button,
  Select,
  useToast,
  AlertDialog,
  AlertDialogBody,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  useDisclosure,
} from "@chakra-ui/react";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";

interface addr {
  mysql?: string;
  redis?: string;
  upload?: string;
  config?: string;
}

interface settings {
  SignUp?: string;
  SSafeUUID?: string;
  TokenDuration?: string;
}

const AdminSettings: React.FC = () => {
  const token = useToken();
  const { get, post } = useApi();
  const [mySQLStatus, setMySQLStatus] = React.useState<number>(0);
  const [redisStatus, setRedisStatus] = React.useState<number>(0);

  const [parameterRes, setParameterRes] = React.useState<addr>();
  const [settings, setSettings] = React.useState<settings>();

  const toast = useToast();

  const {
    isOpen: isOpenDeleteConfirm,
    onOpen: onOpenDeleteConfirm,
    onClose: onCloseDeleteConfirm,
  } = useDisclosure();

  const cancelRef = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn = useRef<HTMLButtonElement>(null);

  const getData = async () => {
    const response = await get("/admin/pingMySQL", {
      headers: { "HRD-token": token },
    });
    if (!!response && response.data.msgCode === 200) {
      setMySQLStatus(response.data.t);
    }

    const response2 = await get("/admin/pingRedis", {
      headers: { "HRD-token": token },
    });
    if (!!response2 && response2.data.msgCode === 200) {
      setRedisStatus(response2.data.t);
    }

    const response3 = await get("/admin/getAddress", {
      headers: { "HRD-token": token },
    });
    if (!!response3 && response3.data.msgCode === 200) {
      setParameterRes(response3.data.t);
    }

    const response4 = await get("/admin/getSettings", {
      headers: { "HRD-token": token },
    });
    if (!!response4 && response4.data.msgCode === 200) {
      setSettings(response4.data.t);
    }
  };
  useEffect(() => {
    if (!!token) {
      getData();
    }
  }, [token]);

  const changeSignUp = async (value: string) => {
    const response = await post(
      "/admin/setSignUpPermission?permission=" + value,
      {},
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 2000,
        position: "top",
      });
      const val = settings?.SignUp === "1" ? "-1" : "1";
      setSettings((prevs) => ({ ...prevs, SignUp: val }));
      return 1;
    } else {
      toast({
        title: `Update Failed`,
        status: "error",
        isClosable: true,
        duration: 2000,
        position: "top",
      });
      return 0;
    }
  };
  const changeSSafe = async (value: string) => {
    const response = await post(
      "/admin/setSSafeUUIDMode?mode=" + value,
      {},
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 2000,
        position: "top",
      });
      const val = settings?.SSafeUUID === "1" ? "-1" : "1";
      setSettings((prevs) => ({ ...prevs, SSafeUUID: val }));
      return 1;
    } else {
      toast({
        title: `Update Failed`,
        status: "error",
        isClosable: true,
        duration: 2000,
        position: "top",
      });
      return 0;
    }
  };
  const changeDuration = async (value: string) => {
    const response = await post(
      "/admin/setTokenDuration?days=" + value,
      {},
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 2000,
        position: "top",
      });
      return 1;
    } else {
      toast({
        title: `Update Failed`,
        status: "error",
        isClosable: true,
        duration: 2000,
        position: "top",
      });
      return 0;
    }
  };

  const stopService = async () => {
    const response = await post(
      "/admin/stop",
      {},
      {
        headers: { "HRD-token": token },
      }
    );
  };

  return (
    <>
      <AlertDialog
        isOpen={isOpenDeleteConfirm}
        leastDestructiveRef={cancelRef}
        onClose={onCloseDeleteConfirm}
      >
        <AlertDialogOverlay>
          <AlertDialogContent>
            <AlertDialogHeader fontSize="lg" fontWeight="bold">
              Stop Service
            </AlertDialogHeader>

            <AlertDialogBody>Are you sure?</AlertDialogBody>

            <AlertDialogFooter>
              <Button ref={cancelRefBtn} onClick={onCloseDeleteConfirm}>
                Cancel
              </Button>
              <Button
                colorScheme="red"
                onClick={() => {
                  stopService();
                }}
                ml={3}
              >
                Terminate
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
      <Center h="calc(100%-4rem)" w={"85vw"}>
        <VStack
          shadow={"lg"}
          w={"55%"}
          rounded={"lg"}
          m={8}
          p={8}
          bg={"white"}
          minH={"80vh"}
        >
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
            p={3}
          >
            Service Status
          </Heading>

          <HStack spacing="24px" alignSelf={"flex-start"}>
            <Box
              w="20vw"
              h="13vh"
              bgGradient="linear(to-r, blue.50, orange.400)"
              alignSelf={"flex-start"}
              borderRadius={20}
            >
              <HStack spacing="24px" h="100%">
                <Box w="8vw" h={"100%"} bg="gray.50" borderRadius={20}>
                  <Image src="/MySQL_ConnectorLogo.png" />
                </Box>
                <Box w="8vw" h="50%" borderRadius={20}>
                  {mySQLStatus === 1 ? (
                    <Alert status="success" h="100%" borderRadius={20}>
                      <AlertIcon />
                      Live
                    </Alert>
                  ) : (
                    <Alert status="warning" h="100%" borderRadius={20}>
                      <AlertIcon />
                      Down
                    </Alert>
                  )}
                </Box>
              </HStack>
            </Box>
            <Box
              w="20vw"
              h="13vh"
              bgGradient="linear(to-r, green.50, red.400)"
              alignSelf={"flex-start"}
              borderRadius={20}
            >
              <HStack spacing="24px" h="100%">
                <Box w="8vw" h={"100%"} bg="gray.50" borderRadius={20}>
                  <Image src="/Redis_Logo.svg.png" />
                </Box>
                <Box w="8vw" h="50%" borderRadius={20}>
                  {redisStatus === 1 ? (
                    <Alert status="success" h="100%" borderRadius={20}>
                      <AlertIcon />
                      Live
                    </Alert>
                  ) : (
                    <Alert status="warning" h="100%" borderRadius={20}>
                      <AlertIcon />
                      Down
                    </Alert>
                  )}
                </Box>
              </HStack>
            </Box>
          </HStack>
          <Divider style={{ marginTop: "2rem" }} />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
            p={3}
          >
            Locations
          </Heading>
          <Stack
            direction={"row"}
            alignSelf="flex-start"
            spacing={"5"}
            w={"100%"}
          >
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">MySQL Addr</Heading>
              <Text mt={4}>{parameterRes?.mysql}</Text>
            </Box>
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">Redis Addr</Heading>
              <Text mt={4}>{parameterRes?.redis}</Text>
            </Box>
          </Stack>
          <Stack
            direction={"row"}
            alignSelf="flex-start"
            spacing={"5"}
            style={{ marginTop: "1rem" }}
            w={"100%"}
          >
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">Upload Addr</Heading>
              <Text mt={4}>{parameterRes?.upload}</Text>
            </Box>
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">Config Addr</Heading>
              <Text mt={4}>{parameterRes?.config}</Text>
            </Box>
          </Stack>
          <Divider style={{ marginTop: "2rem" }} />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
            p={3}
          >
            Settings
          </Heading>
          <Stack
            direction={"row"}
            alignSelf="flex-start"
            spacing={"5"}
            w={"100%"}
          >
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">General</Heading>
              <FormControl
                as={SimpleGrid}
                columns={{ base: 2, lg: 2 }}
                style={{ marginTop: "1rem" }}
              >
                <FormLabel htmlFor="isChecked">Allow User Register:</FormLabel>
                <Switch
                  id="isChecked"
                  size="md"
                  isChecked={settings?.SignUp === "1" ? true : false}
                  onChange={(e) => {
                    const val = e.target.checked ? "1" : "-1";
                    changeSignUp(val);
                  }}
                />
                <FormLabel htmlFor="isChecked">S-Safe UUID Mode:</FormLabel>
                <Switch
                  id="isChecked"
                  size="md"
                  isChecked={settings?.SSafeUUID === "1" ? true : false}
                  onChange={(e) => {
                    const val = e.target.checked ? "1" : "-1";
                    changeSSafe(val);
                  }}
                />
              </FormControl>
              <Divider />
              <FormControl>
                <FormLabel htmlFor="isChecked">
                  SignIn Token Duration:
                </FormLabel>
                <Select
                  id="country"
                  placeholder="Select Duration"
                  value={settings?.TokenDuration}
                  onChange={(e) => {
                    changeDuration(e.target.value);
                    setSettings((prevs) => ({
                      ...prevs,
                      TokenDuration: e.target.value,
                    }));
                  }}
                >
                  <option value="1">1 Day</option>
                  <option value="3">3 Day</option>
                  <option value="5">5 Day</option>
                  <option value="7">7 Day</option>
                  <option value="14">14 Day</option>
                  <option value="30">30 Day</option>
                  <option value="60">60 Day</option>
                  <option value="90">90 Day</option>
                  <option value="120">120 Day</option>
                </Select>
              </FormControl>
            </Box>
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">Power</Heading>
              <Button
                colorScheme="red"
                style={{ marginTop: "1rem" }}
                w={"100%"}
                variant="outline"
                onClick={onOpenDeleteConfirm}
              >
                Stop Service
              </Button>
            </Box>
          </Stack>
        </VStack>
      </Center>
    </>
  );
};

export default AdminSettings;
