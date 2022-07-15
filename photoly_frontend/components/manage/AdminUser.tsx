import {
  Center,
  VStack,
  Heading,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Button,
  Popover,
  PopoverArrow,
  PopoverBody,
  PopoverCloseButton,
  PopoverContent,
  PopoverHeader,
  PopoverTrigger,
  AlertDialog,
  AlertDialogBody,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  useDisclosure,
  useToast,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Text,
  FormControl,
  FormLabel,
  NumberDecrementStepper,
  NumberIncrementStepper,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
***REMOVED*** from "@chakra-ui/react";
import React, { useEffect, useState, useRef ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import { AiOutlineDelete ***REMOVED*** from "react-icons/ai";
import { GrAdd, GrUpdate ***REMOVED*** from "react-icons/gr";

interface user {
  userId: number;
  userName: string;
***REMOVED***

const AdminUser: React.FC = () => {
  const token = useToken();
  const { get, post ***REMOVED*** = useApi();
  const [userList, setUserList] = useState<user[]>();

  const [editUserId, setEditUserId] = useState<number>(0);
  const [editUserName, setEditUserName] = useState<string>("");
  const [delUserId, setDelUserId] = useState<number>();

  const [addUserName, setAddUserName] = useState<string>("");

  const {
    isOpen: isOpenEditModal,
    onOpen: onOpenEditModal,
    onClose: onCloseEditModal,
  ***REMOVED*** = useDisclosure();

  const toast = useToast();
  const {
    isOpen: isOpenDeleteConfirm,
    onOpen: onOpenDeleteConfirm,
    onClose: onCloseDeleteConfirm,
  ***REMOVED*** = useDisclosure();
  const cancelRef = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn = useRef<HTMLButtonElement>(null);

  const {
    isOpen: isOpenAddUser,
    onOpen: onOpenAddUser,
    onClose: onCloseAddUser,
  ***REMOVED*** = useDisclosure();

  const getUser = async () => {
    const response = await get("/user/getAll", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setUserList(response.data.t);
***REMOVED***
  ***REMOVED***;

  const editUser = async (userId: number) => {
    const response = await post(
      "/user/update",
  ***REMOVED***
        userId: userId,
        userName: editUserName,
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
      getUser();
***REMOVED***
  ***REMOVED***;

  const delUser = async () => {
    const response = await post(
      "/user/delete",
  ***REMOVED******REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
        params: {
          userId: delUserId,
***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Delete Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
  ***REMOVED***);
      getUser();
***REMOVED***
  ***REMOVED***;
  const addUser = async () => {
    const response = await post(
      "/user/insert",
  ***REMOVED***
        userName: addUserName,
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
      getUser();
      setAddUserName("");
      onCloseAddUser();
***REMOVED***
  ***REMOVED***;
  useEffect(() => {
    if (!!token) {
      getUser();
***REMOVED***
  ***REMOVED***, [token]);

  return (
    <>
      <Center h="calc(100%-4rem)" w={"90vw"***REMOVED***>
        <VStack
          shadow={"lg"***REMOVED***
          w={"90%"***REMOVED***
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
          >
            Manage Users
            <Popover
              isOpen={isOpenAddUser***REMOVED***
              onClose={() => {
                onCloseAddUser();
 ***REMOVED*****REMOVED******REMOVED***
            >
              <PopoverTrigger>
                <Button
                  colorScheme="teal"
                  variant="ghost"
                  ml={4***REMOVED***
                  rightIcon={<GrAdd />***REMOVED***
                  onClick={() => {
                    onOpenAddUser();
***REMOVED*****REMOVED*****REMOVED******REMOVED***
                >
                  New User
                </Button>
              </PopoverTrigger>
              <PopoverContent>
                <PopoverArrow />
                <PopoverCloseButton />
                <PopoverHeader>Add User</PopoverHeader>
                <PopoverBody>
                  <FormControl>
                    <FormLabel htmlFor="amount">UserName</FormLabel>
                    <Input
                      variant="flushed"
                      placeholder="User Name"
                      value={addUserName***REMOVED***
                      onChange={(event) => setAddUserName(event.target.value)***REMOVED***
                    />
                    <FormLabel htmlFor="amount">Email</FormLabel>
                    <Input
                      variant="flushed"
                      placeholder="User Name"
                      value={addUserName***REMOVED***
                      onChange={(event) => setAddUserName(event.target.value)***REMOVED***
                    />
                    <FormLabel htmlFor="amount">Password</FormLabel>
                    <Input
                      variant="flushed"
                      placeholder="User Name"
                      value={addUserName***REMOVED***
                      onChange={(event) => setAddUserName(event.target.value)***REMOVED***
                    />
                  </FormControl>
                  <Button
                    colorScheme="twitter"
                    variant="outline"
                    width={"100%"***REMOVED***
                    mt={5***REMOVED***
                    onClick={() => {
                      addUser();
  ***REMOVED*****REMOVED*****REMOVED******REMOVED***
                  >
                    Create
                  </Button>
                </PopoverBody>
              </PopoverContent>
            </Popover>
          </Heading>
 ***REMOVED*****REMOVED***userList?.length === 0 ? (
            <Text fontSize="3xl">Nothing Here</Text>
          ) : (
            ""
          )***REMOVED***
          <TableContainer w={"100vw"***REMOVED***>
            <Table variant="simple" size="sm">
              <Thead>
                <Tr>
                  <Th>ID</Th>
                  <Th>Name</Th>
                  <Th>Email</Th>
                  <Th>Date Crd</Th>
                  <Th>Role</Th>
                  <Th>UUID</Th>
                  <Th>Action</Th>
                </Tr>
              </Thead>
              <Tbody>
  ***REMOVED*****REMOVED*****REMOVED***userList?.map((user) => {
                  return (
                    <Tr key={user.userId***REMOVED***>
                      <Td>{user.userId***REMOVED***</Td>
                      <Td>{user.userName***REMOVED***</Td>
                      <Td>
                        <Button
                          leftIcon={<GrUpdate />***REMOVED***
                          onClick={() => {
                            setEditUserName(user.userName);
                            setEditUserId(user.userId);
                            onOpenEditModal();
   ***REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                        ></Button>

                        <Button
                          leftIcon={<AiOutlineDelete />***REMOVED***
                          onClick={() => {
                            setDelUserId(user.userId);
                            onOpenDeleteConfirm();
   ***REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                          disabled={isOpenDeleteConfirm***REMOVED***
                        ></Button>
                      </Td>
                    </Tr>
                  );
   ***REMOVED*****REMOVED***)***REMOVED***
              </Tbody>
            </Table>
          </TableContainer>
        </VStack>

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
                    delUser();
                    onCloseDeleteConfirm();
***REMOVED*****REMOVED*****REMOVED******REMOVED***
                  ml={3***REMOVED***
                >
                  Delete
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialogOverlay>
        </AlertDialog>
        <Modal isOpen={isOpenEditModal***REMOVED*** onClose={onCloseEditModal***REMOVED***>
          <ModalOverlay />
          <ModalContent>
            <ModalHeader>Edit User</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <Input
                variant="flushed"
                placeholder="User New Name"
                value={editUserName***REMOVED***
                onChange={(event) => setEditUserName(event.target.value)***REMOVED***
              />
              <br></br>
            </ModalBody>

            <ModalFooter>
              <Button
                colorScheme="blue"
                variant="ghost"
                mr={3***REMOVED***
                onClick={onCloseEditModal***REMOVED***
              >
                Close
              </Button>
              <Button
                colorScheme="twitter"
                variant="outline"
                onClick={() => {
                  editUser(editUserId);
                  onCloseEditModal();
   ***REMOVED*****REMOVED******REMOVED***
              >
                Confirm
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>
      </Center>
    </>
  );
***REMOVED***;

export default AdminUser;
