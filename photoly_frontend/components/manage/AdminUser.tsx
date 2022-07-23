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
  Box,
  HStack,
  Select,
  Editable,
  EditableInput,
  EditablePreview,
  InputGroup,
  InputRightElement,
  IconButton,
  Stack
***REMOVED*** from "@chakra-ui/react";
import React, { useEffect, useState, useRef ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import { AiOutlineDelete, AiOutlineKey ***REMOVED*** from "react-icons/ai";
import { GrAdd, GrUpdate ***REMOVED*** from "react-icons/gr";
import Pagination from "../Pagination";
import { CloseIcon, SearchIcon ***REMOVED*** from "@chakra-ui/icons";

interface user {
  userId: number;
  userName: string;
  email: string;
  createDate: string;
  role: string;
  uuid: string;
***REMOVED***

const AdminUser: React.FC = () => {
  const token = useToken();
  const { get, post ***REMOVED*** = useApi();
  const [userList, setUserList] = useState<user[]>();
  const [totalPage, setTotalPage] = useState<number>(1);
  const [currPage, setCurrPage] = useState<number>(1);
  const [rowsPerPage, setRowsPerPage] = useState<number>(10);

  const [editUserId, setEditUserId] = useState<number>(0);
  const [editUserPass, setEditUserPass] = useState<string>("");
  const [delUserId, setDelUserId] = useState<number>();

  const [addUserName, setAddUserName] = useState<string>("");
  const [addUserEmail, setAddUserEmail] = useState<string>("");
  const [addUserPass, setAddUserPass] = useState<string>("");
  const [addUserRole, setAddUserRole] = useState<string>("user");

  const [searchContent, setSearchContent] = useState<string>("");
  const [dataSource, setDataSource] = useState<string>("non-search");

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
    const response = await get("/admin/getUserPage", {
      params:{
        "page": currPage,
        "rowsPerPage": rowsPerPage,
  ***REMOVED***,
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setUserList(response.data.t);
      setTotalPage(response.data.pageNum)
***REMOVED***
  ***REMOVED***;

  const searchUser = async () => {
    const response = await get("/admin/searchUserPage", {
      params:{
        "page": currPage,
        "rowsPerPage": rowsPerPage,
        "search": searchContent
  ***REMOVED***,
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setUserList(response.data.t);
      setTotalPage(response.data.pageNum)
***REMOVED***
  ***REMOVED***;

  const editUserEmail = async (userId: number, email:string) => {
    const response = await post(
      "/admin/resetUserEmail",
  ***REMOVED***
        userId: userId,
        email: email,
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
***REMOVED***
    getUser();
  ***REMOVED***;

  const editUsername = async (userId: number, username:string) => {
    const response = await post(
      "/admin/resetUsername",
  ***REMOVED***
        userId: userId,
        userName: username,
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
***REMOVED***
    getUser();
  ***REMOVED***;

  const editUserRole = async (userId: number, role: string) => {
    const response = await post(
      "/admin/resetUserRole",
  ***REMOVED***
        userId: userId,
        role: role,
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
***REMOVED***
    
  ***REMOVED***;

  const editUserPassword = async () => {
    const response = await post(
      "/admin/resetUserPassword",
  ***REMOVED***
        userId: editUserId,
        password: editUserPass,
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
***REMOVED***
    setEditUserId(0)
    setEditUserPass("")
  ***REMOVED***;

  const delUser = async () => {
    const response = await post(
      "/admin/deleteUser",
  ***REMOVED***
        "userId": delUserId
  ***REMOVED***,
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
***REMOVED***
    getUser();
  ***REMOVED***;
  const addUser = async () => {
    const response = await post(
      "/admin/addUser",
  ***REMOVED***
        userName: addUserName,
        email: addUserEmail,
        role: addUserRole,
        password: addUserPass
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
      setAddUserEmail("");
      setAddUserRole("user");
      setAddUserPass("");
      onCloseAddUser();
***REMOVED***
  ***REMOVED***;

  const changeSelection = (num:number) => {
    setCurrPage(num)
  ***REMOVED***

  const cancelSearchMode = () => {
    setDataSource("non-search")
    setSearchContent("")
    setCurrPage(1)
    toast({
      title: `Exit Search Mode`,
      status: "success",
      isClosable: true,
      duration: 2000,
      position: "top",
***REMOVED***);
  ***REMOVED***

  useEffect(() => {
    if (!!token) {
      if (dataSource === "non-search")
***REMOVED***
      getUser();
***REMOVED***else {
      searchUser();
***REMOVED***
***REMOVED***
  ***REMOVED***, [token, currPage, dataSource]);

  return (
    <>
      <Center h="calc(100%-4rem)" w={"85vw"***REMOVED***>
        <VStack
          shadow={"lg"***REMOVED***
          w={"95%"***REMOVED***
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
                      value={addUserEmail***REMOVED***
                      onChange={(event) => setAddUserEmail(event.target.value)***REMOVED***
                    />
                    <FormLabel htmlFor="amount">Password</FormLabel>
                    <Input
                      variant="flushed"
                      placeholder="User Name"
                      value={addUserPass***REMOVED***
                      onChange={(event) => setAddUserPass(event.target.value)***REMOVED***
                    />
                    <FormLabel htmlFor="amount">Role</FormLabel>
                    <Select value={addUserRole***REMOVED*** onChange={(e)=>{setAddUserRole(e.target.value)***REMOVED******REMOVED***>
                      <option value='user'>User</option>
                      <option value='admin'>Admin</option>
                    </Select>
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
            <Stack direction={"row"***REMOVED***>
            <Input
                pr='4.5rem'
                placeholder='Search'
                value={searchContent***REMOVED***
                onChange={(e)=>{setSearchContent(e.target.value)***REMOVED******REMOVED***
              />
              
              <IconButton
                  colorScheme='blue'
                  aria-label='Search database'
                  onClick={()=>{
                    setCurrPage(1)
                    setDataSource("search")
                    searchUser()
***REMOVED*****REMOVED*****REMOVED******REMOVED***
                  icon={<SearchIcon />***REMOVED***
                />
                <IconButton
                  colorScheme='teal'
                  aria-label='Return to Normal'
                  onClick={cancelSearchMode***REMOVED***
                  icon={<CloseIcon />***REMOVED***
                />
            </Stack>
          </Heading>
      ***REMOVED***userList?.length === 0 ? (
            <Text fontSize="3xl">Nothing Here</Text>
          ) : (
            ""
          )***REMOVED***
          <TableContainer w={"100vw"***REMOVED*** style={{display:"flex", flexDirection:"column"***REMOVED******REMOVED***>
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
            ***REMOVED***userList?.map((user) => {
                  return (
                    <Tr key={user.userId***REMOVED***>
                      <Td>{user.userId***REMOVED***</Td>
                      <Td>
                      <Editable defaultValue={user.userName***REMOVED*** onSubmit={(val)=>{editUsername(user.userId, val)***REMOVED******REMOVED***>
                        <EditablePreview />
                        <EditableInput />
                      </Editable>
                        </Td>
                      <Td>
                      <Editable defaultValue={user.email***REMOVED*** onSubmit={(val)=>{editUserEmail(user.userId, val)***REMOVED******REMOVED***>
                        <EditablePreview />
                        <EditableInput />
                      </Editable>
                      </Td>
                      <Td>{user.createDate***REMOVED***</Td>
                      <Td>
                      <Select defaultValue={user.role***REMOVED*** onChange={(e)=>{editUserRole(user.userId, e.target.value)***REMOVED******REMOVED***>
                        <option value='user'>User</option>
                        <option value='admin'>Admin</option>
                      </Select>
                      </Td>
                      <Td>{user.uuid***REMOVED***</Td>
                      <Td>
                        <Button
                          leftIcon={<AiOutlineKey />***REMOVED***
                          onClick={() => {
                            setEditUserPass("");
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
            <Pagination changeSelection={changeSelection***REMOVED*** rowsPerPage={rowsPerPage***REMOVED*** totalPage={totalPage***REMOVED*** page={currPage***REMOVED***/>
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
                Delete User
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
            <ModalHeader>Reset Password</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <Input
                variant="flushed"
                placeholder="New Password"
                value={editUserPass***REMOVED***
                onChange={(event) => setEditUserPass(event.target.value)***REMOVED***
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
                  if (editUserPass.length < 6)
              ***REMOVED***
                    toast({
                      title: 'Password length is too short!',
                      status: 'error',
                      duration: 1500,
                      isClosable: true,
                      position:"top"
  ***REMOVED*****REMOVED*****REMOVED***)
***REMOVED*****REMOVED*****REMOVED***else {
                    editUserPassword();
                    onCloseEditModal();
***REMOVED*****REMOVED*****REMOVED***
                  
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
