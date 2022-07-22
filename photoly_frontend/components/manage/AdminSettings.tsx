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
***REMOVED*** from "@chakra-ui/react";
import React, { useEffect, useState, useRef ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";

interface addr {
  mysql?: string;
  redis?: string;
  upload?: string;
  config?: string;
***REMOVED***

interface settings {
  SignUp?: string;
  SSafeUUID?: string;
  TokenDuration?: string;
***REMOVED***

const AdminSettings: React.FC = () => {
  const token = useToken();
  const { get, post ***REMOVED*** = useApi();
  const [mySQLStatus, setMySQLStatus] = React.useState<number>(0);
  const [redisStatus, setRedisStatus] = React.useState<number>(0);

  const [parameterRes, setParameterRes] = React.useState<addr>();
  const [settings, setSettings] = React.useState<settings>();

  const toast = useToast();

  const {
    isOpen: isOpenDeleteConfirm,
    onOpen: onOpenDeleteConfirm,
    onClose: onCloseDeleteConfirm,
  ***REMOVED*** = useDisclosure();

  const cancelRef = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn = useRef<HTMLButtonElement>(null);

  const getData = async () => {
    const response = await get("/admin/pingMySQL", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setMySQLStatus(response.data.t);
***REMOVED***

    const response2 = await get("/admin/pingRedis", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response2 && response2.data.msgCode === 200) {
      setRedisStatus(response2.data.t);
***REMOVED***

    const response3 = await get("/admin/getAddress", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response3 && response3.data.msgCode === 200) {
      setParameterRes(response3.data.t);
***REMOVED***

    const response4 = await get("/admin/getSettings", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response4 && response4.data.msgCode === 200) {
      setSettings(response4.data.t);
***REMOVED***
  ***REMOVED***;
  useEffect(() => {
    if (!!token) {
      getData();
***REMOVED***
  ***REMOVED***, [token]);

  const changeSignUp = async (value: string) => {
    const response = await post(
      "/admin/setSignUpPermission?permission=" + value,
  ***REMOVED******REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 2000,
        position: "top",
  ***REMOVED***);
      const val = settings?.SignUp === "1" ? "-1" : "1";
      setSettings((prevs) => ({ ...prevs, SignUp: val ***REMOVED***));
      return 1;
***REMOVED*** else {
      toast({
        title: `Update Failed`,
        status: "error",
        isClosable: true,
        duration: 2000,
        position: "top",
  ***REMOVED***);
      return 0;
***REMOVED***
  ***REMOVED***;
  const changeSSafe = async (value: string) => {
    const response = await post(
      "/admin/setSSafeUUIDMode?mode=" + value,
  ***REMOVED******REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 2000,
        position: "top",
  ***REMOVED***);
      const val = settings?.SSafeUUID === "1" ? "-1" : "1";
      setSettings((prevs) => ({ ...prevs, SSafeUUID: val ***REMOVED***));
      return 1;
***REMOVED*** else {
      toast({
        title: `Update Failed`,
        status: "error",
        isClosable: true,
        duration: 2000,
        position: "top",
  ***REMOVED***);
      return 0;
***REMOVED***
  ***REMOVED***;
  const changeDuration = async (value: string) => {
    const response = await post(
      "/admin/setTokenDuration?days=" + value,
  ***REMOVED******REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 2000,
        position: "top",
  ***REMOVED***);
      return 1;
***REMOVED*** else {
      toast({
        title: `Update Failed`,
        status: "error",
        isClosable: true,
        duration: 2000,
        position: "top",
  ***REMOVED***);
      return 0;
***REMOVED***
  ***REMOVED***;

  const stopService = async () => {
    const response = await post(
      "/admin/stop",
  ***REMOVED******REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
  ***REMOVED***;

  return (
    <>
      <AlertDialog
        isOpen={isOpenDeleteConfirm***REMOVED***
        leastDestructiveRef={cancelRef***REMOVED***
        onClose={onCloseDeleteConfirm***REMOVED***
      >
        <AlertDialogOverlay>
          <AlertDialogContent>
            <AlertDialogHeader fontSize="lg" fontWeight="bold">
              Stop Service
            </AlertDialogHeader>

            <AlertDialogBody>Are you sure?</AlertDialogBody>

            <AlertDialogFooter>
              <Button ref={cancelRefBtn***REMOVED*** onClick={onCloseDeleteConfirm***REMOVED***>
                Cancel
              </Button>
              <Button
                colorScheme="red"
                onClick={() => {
                  stopService();
   ***REMOVED*****REMOVED******REMOVED***
                ml={3***REMOVED***
              >
                Terminate
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
      <Center h="calc(100%-4rem)" w={"85vw"***REMOVED***>
        <VStack
          shadow={"lg"***REMOVED***
          w={"55%"***REMOVED***
          rounded={"lg"***REMOVED***
          m={8***REMOVED***
          p={8***REMOVED***
          bg={"white"***REMOVED***
          minH={"80vh"***REMOVED***
        >
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"***REMOVED***
            alignSelf={"flex-start"***REMOVED***
            p={3***REMOVED***
          >
            Service Status
          </Heading>

          <HStack spacing="24px" alignSelf={"flex-start"***REMOVED***>
            <Box
              w="20vw"
              h="13vh"
              bgGradient="linear(to-r, blue.50, orange.400)"
              alignSelf={"flex-start"***REMOVED***
              borderRadius={20***REMOVED***
            >
              <HStack spacing="24px" h="100%">
                <Box w="8vw" h={"100%"***REMOVED*** bg="gray.50" borderRadius={20***REMOVED***>
                  <Image src="/MySQL_ConnectorLogo.png" />
                </Box>
                <Box w="8vw" h="50%" borderRadius={20***REMOVED***>
              ***REMOVED***mySQLStatus === 1 ? (
                    <Alert status="success" h="100%" borderRadius={20***REMOVED***>
                      <AlertIcon />
                      Live
                    </Alert>
                  ) : (
                    <Alert status="warning" h="100%" borderRadius={20***REMOVED***>
                      <AlertIcon />
                      Down
                    </Alert>
                  )***REMOVED***
                </Box>
              </HStack>
            </Box>
            <Box
              w="20vw"
              h="13vh"
              bgGradient="linear(to-r, green.50, red.400)"
              alignSelf={"flex-start"***REMOVED***
              borderRadius={20***REMOVED***
            >
              <HStack spacing="24px" h="100%">
                <Box w="8vw" h={"100%"***REMOVED*** bg="gray.50" borderRadius={20***REMOVED***>
                  <Image src="/Redis_Logo.svg.png" />
                </Box>
                <Box w="8vw" h="50%" borderRadius={20***REMOVED***>
              ***REMOVED***redisStatus === 1 ? (
                    <Alert status="success" h="100%" borderRadius={20***REMOVED***>
                      <AlertIcon />
                      Live
                    </Alert>
                  ) : (
                    <Alert status="warning" h="100%" borderRadius={20***REMOVED***>
                      <AlertIcon />
                      Down
                    </Alert>
                  )***REMOVED***
                </Box>
              </HStack>
            </Box>
          </HStack>
          <Divider style={{ marginTop: "2rem" ***REMOVED******REMOVED*** />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"***REMOVED***
            alignSelf={"flex-start"***REMOVED***
            p={3***REMOVED***
          >
            Locations
          </Heading>
          <Stack
            direction={"row"***REMOVED***
            alignSelf="flex-start"
            spacing={"5"***REMOVED***
            w={"100%"***REMOVED***
          >
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">MySQL Addr</Heading>
              <Text mt={4***REMOVED***>{parameterRes?.mysql***REMOVED***</Text>
            </Box>
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">Redis Addr</Heading>
              <Text mt={4***REMOVED***>{parameterRes?.redis***REMOVED***</Text>
            </Box>
          </Stack>
          <Stack
            direction={"row"***REMOVED***
            alignSelf="flex-start"
            spacing={"5"***REMOVED***
            style={{ marginTop: "1rem" ***REMOVED******REMOVED***
            w={"100%"***REMOVED***
          >
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">Upload Addr</Heading>
              <Text mt={4***REMOVED***>{parameterRes?.upload***REMOVED***</Text>
            </Box>
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">Config Addr</Heading>
              <Text mt={4***REMOVED***>{parameterRes?.config***REMOVED***</Text>
            </Box>
          </Stack>
          <Divider style={{ marginTop: "2rem" ***REMOVED******REMOVED*** />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"***REMOVED***
            alignSelf={"flex-start"***REMOVED***
            p={3***REMOVED***
          >
            Settings
          </Heading>
          <Stack
            direction={"row"***REMOVED***
            alignSelf="flex-start"
            spacing={"5"***REMOVED***
            w={"100%"***REMOVED***
          >
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">General</Heading>
              <FormControl
                as={SimpleGrid***REMOVED***
                columns={{ base: 2, lg: 2 ***REMOVED******REMOVED***
                style={{ marginTop: "1rem" ***REMOVED******REMOVED***
              >
                <FormLabel htmlFor="isChecked">Allow User Register:</FormLabel>
                <Switch
                  id="isChecked"
                  size="md"
                  isChecked={settings?.SignUp === "1" ? true : false***REMOVED***
                  onChange={(e) => {
                    const val = e.target.checked ? "1" : "-1";
                    changeSignUp(val);
***REMOVED*****REMOVED*****REMOVED******REMOVED***
                />
                <FormLabel htmlFor="isChecked">S-Safe UUID Mode:</FormLabel>
                <Switch
                  id="isChecked"
                  size="md"
                  isChecked={settings?.SSafeUUID === "1" ? true : false***REMOVED***
                  onChange={(e) => {
                    const val = e.target.checked ? "1" : "-1";
                    changeSSafe(val);
***REMOVED*****REMOVED*****REMOVED******REMOVED***
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
                  value={settings?.TokenDuration***REMOVED***
                  onChange={(e) => {
                    changeDuration(e.target.value);
                    setSettings((prevs) => ({
                      ...prevs,
                      TokenDuration: e.target.value,
  ***REMOVED*****REMOVED*****REMOVED***));
***REMOVED*****REMOVED*****REMOVED******REMOVED***
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
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">Power</Heading>
              <Button
                colorScheme="red"
                style={{ marginTop: "1rem" ***REMOVED******REMOVED***
                w={"100%"***REMOVED***
                variant="outline"
                onClick={onOpenDeleteConfirm***REMOVED***
              >
                Stop Service
              </Button>
            </Box>
          </Stack>
        </VStack>
      </Center>
    </>
  );
***REMOVED***;

export default AdminSettings;
