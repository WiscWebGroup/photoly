import {
  Avatar,
  Center,
  Divider,
  Heading,
  HStack,
  IconButton,
  Text,
  useDisclosure,
  VStack,
  Button,
  Grid,
  GridItem,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Stack,
  useToast,
  AlertDialog,
  AlertDialogBody,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogContent,
  AlertDialogOverlay,
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverHeader,
  PopoverBody,
  PopoverArrow,
  PopoverCloseButton,
  Checkbox,
  CheckboxGroup,
  useCheckboxGroup,
  Accordion,
  AccordionItem,
  AccordionButton,
  AccordionPanel,
  AccordionIcon,
  Box,
***REMOVED*** from "@chakra-ui/react";
import { AiOutlineEdit ***REMOVED*** from "react-icons/ai";
import React, { useEffect, useState, useRef ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import ChangeInfoDrawer from "../ChangeInfoDrawer";
import { AiOutlineCopy, AiOutlineDelete ***REMOVED*** from "react-icons/ai";
import { GrUpdate, GrAdd ***REMOVED*** from "react-icons/gr";

interface userInfo {
  userId: number;
  userName: string;
  email: string;
  createDate: string;
  role: string;
  uuid: string;
***REMOVED***

interface cred {
  credId: number;
  userId: number;
  token: string;
  authorization: string;
***REMOVED***

const MyProfile: React.FC = () => {
  const token = useToken();
  const { get, post ***REMOVED*** = useApi();
  const [info, setInfo] = useState<userInfo>();
  const { isOpen, onOpen, onClose ***REMOVED*** = useDisclosure();

  const [credList, setCredList] = useState<cred[]>();
  const { value, setValue ***REMOVED*** = useCheckboxGroup({***REMOVED***);

  const [delCredId, setDelCredId] = useState<number>();

  const toast = useToast();
  const {
    isOpen: isOpenDeleteConfirm,
    onOpen: onOpenDeleteConfirm,
    onClose: onCloseDeleteConfirm,
  ***REMOVED*** = useDisclosure();
  const cancelRef = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn = useRef<HTMLButtonElement>(null);

  const {
    isOpen: isOpenAdd,
    onOpen: onOpenAdd,
    onClose: onCloseAdd,
  ***REMOVED*** = useDisclosure();

  const getInfo = async () => {
    const response = await get("/user/getInfo", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setInfo(response.data.t);
***REMOVED***
  ***REMOVED***;

  const getCred = async () => {
    const response = await get("/cred/query", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setCredList(response.data.t);
***REMOVED***
  ***REMOVED***;

  const delCred = async () => {
    const response = await post(
      "/cred/delete",
  ***REMOVED******REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
        params: {
          credId: delCredId,
***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Delete Successful`,
        status: "success",
        isClosable: true,
        position: "top",
        duration: 3000,
  ***REMOVED***);
      getCred();
***REMOVED***
  ***REMOVED***;

  const getEditCred = (auth: string) => {
    var lst = [];
    if (auth.includes("C")) {
      lst?.push("C");
***REMOVED***
    if (auth.includes("R")) {
      lst?.push("R");
***REMOVED***
    if (auth.includes("D")) {
      lst?.push("D");
***REMOVED***
    setValue(lst);
  ***REMOVED***;

  const editCred = async (credId: number) => {
    const response = await post(
      "/cred/update",
  ***REMOVED***
        credId: credId,
        authorization: value.join(),
  ***REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
  ***REMOVED***);
      getCred();
***REMOVED***
  ***REMOVED***;

  const addCred = async () => {
    const response = await post(
      "/cred/create",
  ***REMOVED***
        authorization: value.join(),
  ***REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Create Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
  ***REMOVED***);
      getCred();
***REMOVED***
  ***REMOVED***;

  const copyToken = (token: string) => {
    navigator.clipboard.writeText(token);
    toast({
      title: `Copied to your Clipboard`,
      status: "success",
      isClosable: true,
      duration: 3000,
      position: "top",
***REMOVED***);
  ***REMOVED***;

  useEffect(() => {
    if (!!token) {
      getInfo();
      getCred();
***REMOVED***
  ***REMOVED***, [token]);

  return (
    <>
      <ChangeInfoDrawer isOpen={isOpen***REMOVED*** onClose={onClose***REMOVED*** />
      <Center h="calc(100%-4rem)" w={"85vw"***REMOVED***>
        <VStack shadow={"lg"***REMOVED*** w={"55%"***REMOVED*** rounded={"lg"***REMOVED*** m={8***REMOVED*** p={8***REMOVED*** bg={"white"***REMOVED***>
          <HStack
            justifyContent={"space-evenly"***REMOVED***
            w={"100%"***REMOVED***
            h="25vh"
            style={{
              backgroundImage: "linear-gradient(to right, #B2F5EA, #BEE3F8)",
    ***REMOVED******REMOVED***
            rounded={"lg"***REMOVED***
          >
            <VStack>
              <Avatar
                borderRadius="full"
                h={24***REMOVED***
                w={24***REMOVED***
                src={`user/getAvatar/${token***REMOVED***`***REMOVED***
              />
              <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED*** color={"#97266D"***REMOVED***>
  ***REMOVED*****REMOVED*****REMOVED***info?.userName***REMOVED***
              </Text>
            </VStack>
          </HStack>

          <Divider />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"***REMOVED***
            alignSelf={"flex-start"***REMOVED***
          >
            Personal Profile
          </Heading>
          <Grid
            h="200px"
            templateRows="repeat(2, 1fr)"
            templateColumns="repeat(5, 1fr)"
            gap={4***REMOVED***
            width="100%"
          >
            <GridItem colSpan={2***REMOVED*** padding={3***REMOVED*** bg={"orange.200"***REMOVED*** rounded={"lg"***REMOVED***>
              <Heading
                as="h6"
                size="md"
                fontWeight={"450"***REMOVED***
                color="RGBA(0, 0, 0, 0.80)"
              >
                Name
              </Heading>
              <Text marginTop={2***REMOVED*** marginLeft={1***REMOVED***>
  ***REMOVED*****REMOVED*****REMOVED***info?.userName***REMOVED***
              </Text>
            </GridItem>
            <GridItem colSpan={2***REMOVED*** padding={3***REMOVED*** bg="orange.200" rounded={"lg"***REMOVED***>
              <Heading
                as="h6"
                size="md"
                fontWeight={"450"***REMOVED***
                color="RGBA(0, 0, 0, 0.80)"
              >
                Register Time
              </Heading>
              <Text marginTop={2***REMOVED*** marginLeft={1***REMOVED***>
  ***REMOVED*****REMOVED*****REMOVED***info?.createDate.substr(0, 10)***REMOVED***
              </Text>
            </GridItem>
            <GridItem
              rowSpan={2***REMOVED***
              padding={3***REMOVED***
              colSpan={1***REMOVED***
              bg={"purple.50"***REMOVED***
              rounded={"lg"***REMOVED***
            >
              <Heading
                as="h6"
                size="md"
                fontWeight={"450"***REMOVED***
                color="RGBA(0, 0, 0, 0.80)"
              >
                Edit Profile
              </Heading>
              <IconButton
                variant={"ghost"***REMOVED***
                aria-label={"Edit profile"***REMOVED***
                icon={<AiOutlineEdit />***REMOVED***
                onClick={onOpen***REMOVED***
                size={"lg"***REMOVED***
                mt={4***REMOVED***
                padding={5***REMOVED***
                w={"100%"***REMOVED***
              />
            </GridItem>
            <GridItem colSpan={4***REMOVED*** padding={3***REMOVED*** bg={"teal.100"***REMOVED*** rounded={"lg"***REMOVED***>
              <Heading as="h6" size="md" fontWeight={"450"***REMOVED***>
                Email
              </Heading>
              <Text marginTop={2***REMOVED*** marginLeft={1***REMOVED***>
  ***REMOVED*****REMOVED*****REMOVED***info?.email***REMOVED***
              </Text>
            </GridItem>
          </Grid>

          <Divider />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"***REMOVED***
            alignSelf={"flex-start"***REMOVED***
          >
            API
            <Popover
              isOpen={isOpenAdd***REMOVED***
              onClose={() => {
                onCloseAdd();
 ***REMOVED*****REMOVED******REMOVED***
            >
              <PopoverTrigger>
                <Button
                  colorScheme="teal"
                  variant="ghost"
                  ml={4***REMOVED***
                  rightIcon={<GrAdd />***REMOVED***
                  onClick={() => {
                    onOpenAdd();
                    setValue([]);
***REMOVED*****REMOVED*****REMOVED******REMOVED***
                >
                  New API
                </Button>
              </PopoverTrigger>
              <PopoverContent>
                <PopoverArrow />
                <PopoverCloseButton />
                <PopoverHeader>Add API</PopoverHeader>
                <PopoverBody>
                  <CheckboxGroup
                    colorScheme="green"
                    defaultValue={[]***REMOVED***
                    value={value***REMOVED***
                    onChange={(e) => {
                      setValue(e);
  ***REMOVED*****REMOVED*****REMOVED******REMOVED***
                  >
                    <Stack spacing={[1, 5]***REMOVED*** direction={["column", "row"]***REMOVED***>
                      <Checkbox value="C">Upload</Checkbox>
                      <Checkbox value="R">Read</Checkbox>
                      <Checkbox value="D">Delete</Checkbox>
                    </Stack>
                  </CheckboxGroup>
                  <Button
                    colorScheme="twitter"
                    variant="outline"
                    width={"100%"***REMOVED***
                    mt={5***REMOVED***
                    onClick={() => {
                      addCred();
                      onCloseAdd();
  ***REMOVED*****REMOVED*****REMOVED******REMOVED***
                  >
                    Create
                  </Button>
                </PopoverBody>
              </PopoverContent>
            </Popover>
          </Heading>
          <Stack width={"100%"***REMOVED***>
            <TableContainer w={"100vw"***REMOVED***>
              <Table variant="simple">
                <Thead>
                  <Tr>
                    <Th>ID</Th>
                    <Th>Authorization</Th>
                    <Th>Token</Th>
                    <Th>Action</Th>
                  </Tr>
                </Thead>
                <Tbody>
    ***REMOVED*****REMOVED*****REMOVED***credList?.map((cred) => {
                    return (
                      <Tr key={cred.credId***REMOVED***>
                        <Td>{cred.credId***REMOVED***</Td>
                        <Td>
       ***REMOVED*****REMOVED*****REMOVED*****REMOVED***cred.authorization
                            .replace("C", "Upload ")
                            .replace("R", "Read ")
                            .replace("D", "Delete")***REMOVED***
                        </Td>
                        <Td>{cred.token***REMOVED***</Td>
                        <Td>
                          <Button
                            leftIcon={<AiOutlineCopy />***REMOVED***
                            onClick={() => {
                              copyToken(cred.token);
***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                          ></Button>
                          <Popover>
                            <PopoverTrigger>
                              <Button
                                leftIcon={<GrUpdate />***REMOVED***
                                onClick={() => {
                                  getEditCred(cred.authorization);
    ***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                              ></Button>
                            </PopoverTrigger>
                            <PopoverContent>
                              <PopoverArrow />
                              <PopoverCloseButton />
                              <PopoverHeader>Update Auth</PopoverHeader>
                              <PopoverBody>
                                <CheckboxGroup
                                  colorScheme="green"
                                  value={value***REMOVED***
                                  onChange={(e) => {
                                    setValue(e);
 ***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                                >
                                  <Stack
                                    spacing={[1, 5]***REMOVED***
                                    direction={["column", "row"]***REMOVED***
                                  >
                                    <Checkbox value="C">Upload</Checkbox>
                                    <Checkbox value="R">Read</Checkbox>
                                    <Checkbox value="D">Delete</Checkbox>
                                  </Stack>
                                </CheckboxGroup>
                                <Button
                                  colorScheme="twitter"
                                  variant="outline"
                                  width={"100%"***REMOVED***
                                  mt={5***REMOVED***
                                  onClick={() => {
                                    editCred(cred.credId);
 ***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                                >
                                  Confirm
                                </Button>
                              </PopoverBody>
                            </PopoverContent>
                          </Popover>

                          <Button
                            leftIcon={<AiOutlineDelete />***REMOVED***
                            onClick={() => {
                              setDelCredId(cred.credId);
                              onOpenDeleteConfirm();
***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                            disabled={isOpenDeleteConfirm***REMOVED***
                          ></Button>
                        </Td>
                      </Tr>
                    );
***REMOVED*****REMOVED*****REMOVED***)***REMOVED***
                </Tbody>
              </Table>
            </TableContainer>
            <Heading
              as="h6"
              size="md"
              fontWeight={"450"***REMOVED***
              alignSelf={"flex-start"***REMOVED***
              style={{ marginTop: 20 ***REMOVED******REMOVED***
            >
              API Documents
            </Heading>
            <Accordion
              defaultIndex={[]***REMOVED***
              allowMultiple
              style={{ marginTop: 20 ***REMOVED******REMOVED***
            >
              <AccordionItem>
                <h2>
                  <AccordionButton>
                    <Box flex="1" textAlign="left">
                      API functionalities
                    </Box>
                    <AccordionIcon />
                  </AccordionButton>
                </h2>
                <AccordionPanel pb={4***REMOVED***>
                  C: Able to upload photo (Create) <br></br>R: Able to query
                  namespace, photos in namespaces and render the photo (Read)
                  <br></br> D: Able to delete photo (Delete)
                </AccordionPanel>
              </AccordionItem>
              <AccordionItem>
                <h2>
                  <AccordionButton>
                    <Box flex="1" textAlign="left">
                      How to use API in external build
                    </Box>
                    <AccordionIcon />
                  </AccordionButton>
                </h2>
                <AccordionPanel pb={4***REMOVED***>
                  <Text fontSize="xl">C1 Upload Photo - POST:</Text>
                  <br></br>
                  Addr: server_path/cred/uploadPhoto/YOUR_TOKEN
                  <br></br>
                  Takein: file:MultipartFile | photo:JSONText containing
                  nsId(num) and visibility(num)
                  <br></br>
                  Return: a string that is the photo's uuid
                  <br></br>
                  <br></br>
                  <Text fontSize="xl">C2 Upload Photos - POST:</Text>
                  <br></br>
                  Addr: server_path/cred/uploadPhotos/YOUR_TOKEN
                  <br></br>
                  Takein: files:MultipartFile[] | photos:JSONText containing
                  list of photos( nsId(num) and visibility(num) )<br></br>
                  Return: a list of string of photo uuids
                  <br></br>
                  <br></br>
                  <Text fontSize="xl">D Delete Photo - POST:</Text>
                  <br></br>
                  Addr: server_path/cred/deletePhoto/YOUR_TOKEN
                  <br></br>
                  Takein: uuid: str, photo's uuid
                  <br></br>
                  Return: an int, 1 if successful
                  <br></br>
                  <br></br>
                  <Text fontSize="xl">R1 Render Photo - POST:</Text>
                  <br></br>
                  Addr: server_path/cred/render/YOUR_TOKEN
                  <br></br>
                  Takein: uuid: str, photo's uuid
                  <br></br>
                  Return: a byte[] that is the image (could be seen directly in
                  browser url)
                  <br></br>
                  <br></br>
                  <Text fontSize="xl">R2 Get RootNS - GET:</Text>
                  <br></br>
                  Addr: server_path/cred/queryRootNamespace/YOUR_TOKEN
                  <br></br>Takein: None
                  <br></br>
                  Return: an JSON object that is the Root namespace
                  <br></br>
                  <br></br>
                  <Text fontSize="xl">R3 Get NS - GET:</Text>
                  <br></br>
                  Addr: server_path/cred/queryNamespaces/YOUR_TOKEN
                  <br></br>Takein: parentId: int, the parentId of ns
                  <br></br>
                  Return: an JSON object that is the list of namespaces under
                  parentId
                  <br></br>
                  <br></br>
                  <Text fontSize="xl">R4 Get Photo List - GET:</Text>
                  <br></br>
                  Addr: server_path/cred/queryPhotoList/YOUR_TOKEN
                  <br></br>Takein: nsId: int, the namespace Id
                  <br></br>
                  Return: an JSON object that is the list of photos' meta info
                  <br></br>
                  <br></br>
                </AccordionPanel>
              </AccordionItem>
            </Accordion>
            <Divider style={{ marginTop: 20 ***REMOVED******REMOVED*** />
            <Heading
              as="h6"
              size="md"
              fontWeight={"450"***REMOVED***
              alignSelf={"flex-start"***REMOVED***
            >
              Other
            </Heading>
            <Button colorScheme="red" variant="ghost" w={"100%"***REMOVED***>
              Delete my Account
            </Button>
          </Stack>
        </VStack>
      </Center>
      <AlertDialog
        isOpen={isOpenDeleteConfirm***REMOVED***
        leastDestructiveRef={cancelRef***REMOVED***
        onClose={onCloseDeleteConfirm***REMOVED***
      >
        <AlertDialogOverlay>
          <AlertDialogContent>
            <AlertDialogHeader fontSize="lg" fontWeight="bold">
              Delete API
            </AlertDialogHeader>

            <AlertDialogBody>
              Are you sure? You can't undo this action afterwards.
            </AlertDialogBody>

            <AlertDialogFooter>
              <Button ref={cancelRefBtn***REMOVED*** onClick={onCloseDeleteConfirm***REMOVED***>
                Cancel
              </Button>
              <Button
                colorScheme="red"
                onClick={() => {
                  delCred();
                  onCloseDeleteConfirm();
   ***REMOVED*****REMOVED******REMOVED***
                ml={3***REMOVED***
              >
                Delete
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
    </>
  );
***REMOVED***;
export default MyProfile;
