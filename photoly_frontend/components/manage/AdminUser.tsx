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
} from "@chakra-ui/react";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import { AiOutlineDelete, AiOutlineKey } from "react-icons/ai";
import { GrAdd, GrUpdate } from "react-icons/gr";
import Pagination from "../Pagination";
import { CloseIcon, SearchIcon } from "@chakra-ui/icons";

interface user {
  userId: number;
  userName: string;
  email: string;
  createDate: string;
  role: string;
  uuid: string;
}

const AdminUser: React.FC = () => {
  const token = useToken();
  const { get, post } = useApi();
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
  } = useDisclosure();

  const toast = useToast();
  const {
    isOpen: isOpenDeleteConfirm,
    onOpen: onOpenDeleteConfirm,
    onClose: onCloseDeleteConfirm,
  } = useDisclosure();
  const cancelRef = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn = useRef<HTMLButtonElement>(null);

  const {
    isOpen: isOpenAddUser,
    onOpen: onOpenAddUser,
    onClose: onCloseAddUser,
  } = useDisclosure();

  const getUser = async () => {
    const response = await get("/admin/getUserPage", {
      params:{
        "page": currPage,
        "rowsPerPage": rowsPerPage,
      },
      headers: { "HRD-token": token },
    });
    if (!!response && response.data.msgCode === 200) {
      setUserList(response.data.t);
      setTotalPage(response.data.pageNum)
    }
  };

  const searchUser = async () => {
    const response = await get("/admin/searchUserPage", {
      params:{
        "page": currPage,
        "rowsPerPage": rowsPerPage,
        "search": searchContent
      },
      headers: { "HRD-token": token },
    });
    if (!!response && response.data.msgCode === 200) {
      setUserList(response.data.t);
      setTotalPage(response.data.pageNum)
    }
  };

  const editUserEmail = async (userId: number, email:string) => {
    const response = await post(
      "/admin/resetUserEmail",
      {
        userId: userId,
        email: email,
      },
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
      });
    }
    getUser();
  };

  const editUsername = async (userId: number, username:string) => {
    const response = await post(
      "/admin/resetUsername",
      {
        userId: userId,
        userName: username,
      },
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
      });
    }
    getUser();
  };

  const editUserRole = async (userId: number, role: string) => {
    const response = await post(
      "/admin/resetUserRole",
      {
        userId: userId,
        role: role,
      },
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
      });
    }
    
  };

  const editUserPassword = async () => {
    const response = await post(
      "/admin/resetUserPassword",
      {
        userId: editUserId,
        password: editUserPass,
      },
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
      });
    }
    setEditUserId(0)
    setEditUserPass("")
  };

  const delUser = async () => {
    const response = await post(
      "/admin/deleteUser",
      {
        "userId": delUserId
      },
      {
        headers: { "HRD-token": token },
        params: {
          userId: delUserId,
        },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Delete Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
      });
    }
    getUser();
  };
  const addUser = async () => {
    const response = await post(
      "/admin/addUser",
      {
        userName: addUserName,
        email: addUserEmail,
        role: addUserRole,
        password: addUserPass
      },
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Create Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
      });
      getUser();
      setAddUserName("");
      setAddUserEmail("");
      setAddUserRole("user");
      setAddUserPass("");
      onCloseAddUser();
    }
  };

  const changeSelection = (num:number) => {
    setCurrPage(num)
  }

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
    });
  }

  useEffect(() => {
    if (!!token) {
      if (dataSource === "non-search")
    {
      getUser();
    }else {
      searchUser();
    }
    }
  }, [token, currPage, dataSource]);

  return (
    <>
      <Center h="calc(100%-4rem)" w={"85vw"}>
        <VStack
          shadow={"lg"}
          w={"95%"}
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
          >
            Manage Users
            <Popover
              isOpen={isOpenAddUser}
              onClose={() => {
                onCloseAddUser();
              }}
            >
              <PopoverTrigger>
                <Button
                  colorScheme="teal"
                  variant="ghost"
                  ml={4}
                  rightIcon={<GrAdd />}
                  onClick={() => {
                    onOpenAddUser();
                  }}
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
                      value={addUserName}
                      onChange={(event) => setAddUserName(event.target.value)}
                    />
                    <FormLabel htmlFor="amount">Email</FormLabel>
                    <Input
                      variant="flushed"
                      placeholder="User Name"
                      value={addUserEmail}
                      onChange={(event) => setAddUserEmail(event.target.value)}
                    />
                    <FormLabel htmlFor="amount">Password</FormLabel>
                    <Input
                      variant="flushed"
                      placeholder="User Name"
                      value={addUserPass}
                      onChange={(event) => setAddUserPass(event.target.value)}
                    />
                    <FormLabel htmlFor="amount">Role</FormLabel>
                    <Select value={addUserRole} onChange={(e)=>{setAddUserRole(e.target.value)}}>
                      <option value='user'>User</option>
                      <option value='admin'>Admin</option>
                    </Select>
                  </FormControl>
                  <Button
                    colorScheme="twitter"
                    variant="outline"
                    width={"100%"}
                    mt={5}
                    onClick={() => {
                      addUser();
                    }}
                  >
                    Create
                  </Button>
                </PopoverBody>
              </PopoverContent>
            </Popover>
            <Stack direction={"row"}>
            <Input
                pr='4.5rem'
                placeholder='Search'
                value={searchContent}
                onChange={(e)=>{setSearchContent(e.target.value)}}
              />
              
              <IconButton
                  colorScheme='blue'
                  aria-label='Search database'
                  onClick={()=>{
                    setCurrPage(1)
                    setDataSource("search")
                    searchUser()
                  }}
                  icon={<SearchIcon />}
                />
                <IconButton
                  colorScheme='teal'
                  aria-label='Return to Normal'
                  onClick={cancelSearchMode}
                  icon={<CloseIcon />}
                />
            </Stack>
          </Heading>
          {userList?.length === 0 ? (
            <Text fontSize="3xl">Nothing Here</Text>
          ) : (
            ""
          )}
          <TableContainer w={"100vw"} style={{display:"flex", flexDirection:"column"}}>
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
                {userList?.map((user) => {
                  return (
                    <Tr key={user.userId}>
                      <Td>{user.userId}</Td>
                      <Td>
                      <Editable defaultValue={user.userName} onSubmit={(val)=>{editUsername(user.userId, val)}}>
                        <EditablePreview />
                        <EditableInput />
                      </Editable>
                        </Td>
                      <Td>
                      <Editable defaultValue={user.email} onSubmit={(val)=>{editUserEmail(user.userId, val)}}>
                        <EditablePreview />
                        <EditableInput />
                      </Editable>
                      </Td>
                      <Td>{user.createDate}</Td>
                      <Td>
                      <Select defaultValue={user.role} onChange={(e)=>{editUserRole(user.userId, e.target.value)}}>
                        <option value='user'>User</option>
                        <option value='admin'>Admin</option>
                      </Select>
                      </Td>
                      <Td>{user.uuid}</Td>
                      <Td>
                        <Button
                          leftIcon={<AiOutlineKey />}
                          onClick={() => {
                            setEditUserPass("");
                            setEditUserId(user.userId);
                            onOpenEditModal();
                          }}
                        ></Button>

                        <Button
                          leftIcon={<AiOutlineDelete />}
                          onClick={() => {
                            setDelUserId(user.userId);
                            onOpenDeleteConfirm();
                          }}
                          disabled={isOpenDeleteConfirm}
                        ></Button>
                      </Td>
                    </Tr>
                  );
                })}
                
              </Tbody>
            </Table>
            <Pagination changeSelection={changeSelection} rowsPerPage={rowsPerPage} totalPage={totalPage} page={currPage}/>
          </TableContainer>
          
        </VStack>
        
        

        <AlertDialog
          isOpen={isOpenDeleteConfirm}
          leastDestructiveRef={cancelRef}
          onClose={onCloseDeleteConfirm}
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
                <Button ref={cancelRefBtn} onClick={onCloseDeleteConfirm}>
                  Cancel
                </Button>
                <Button
                  colorScheme="red"
                  onClick={() => {
                    delUser();
                    onCloseDeleteConfirm();
                  }}
                  ml={3}
                >
                  Delete
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialogOverlay>
        </AlertDialog>
        <Modal isOpen={isOpenEditModal} onClose={onCloseEditModal}>
          <ModalOverlay />
          <ModalContent>
            <ModalHeader>Reset Password</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <Input
                variant="flushed"
                placeholder="New Password"
                value={editUserPass}
                onChange={(event) => setEditUserPass(event.target.value)}
              />
              <br></br>
            </ModalBody>

            <ModalFooter>
              <Button
                colorScheme="blue"
                variant="ghost"
                mr={3}
                onClick={onCloseEditModal}
              >
                Close
              </Button>
              <Button
                colorScheme="twitter"
                variant="outline"
                onClick={() => {
                  if (editUserPass.length < 6)
                  {
                    toast({
                      title: 'Password length is too short!',
                      status: 'error',
                      duration: 1500,
                      isClosable: true,
                      position:"top"
                    })
                  }else {
                    editUserPassword();
                    onCloseEditModal();
                  }
                  
                }}
              >
                Confirm
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>
      </Center>
    </>
  );
};

export default AdminUser;
