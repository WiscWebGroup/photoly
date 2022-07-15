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
} from "@chakra-ui/react";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import { AiOutlineDelete } from "react-icons/ai";
import { GrAdd, GrUpdate } from "react-icons/gr";

interface user {
  userId: number;
  userName: string;
}

const AdminUser: React.FC = () => {
  const token = useToken();
  const { get, post } = useApi();
  const [userList, setUserList] = useState<user[]>();

  const [editUserId, setEditUserId] = useState<number>(0);
  const [editUserName, setEditUserName] = useState<string>("");
  const [delUserId, setDelUserId] = useState<number>();

  const [addUserName, setAddUserName] = useState<string>("");

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
    const response = await get("/user/getAll", {
      headers: { "HRD-token": token },
    });
    if (!!response && response.data.msgCode === 200) {
      setUserList(response.data.t);
    }
  };

  const editUser = async (userId: number) => {
    const response = await post(
      "/user/update",
      {
        userId: userId,
        userName: editUserName,
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
      getUser();
    }
  };

  const delUser = async () => {
    const response = await post(
      "/user/delete",
      {},
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
      getUser();
    }
  };
  const addUser = async () => {
    const response = await post(
      "/user/insert",
      {
        userName: addUserName,
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
      onCloseAddUser();
    }
  };
  useEffect(() => {
    if (!!token) {
      getUser();
    }
  }, [token]);

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
                      value={addUserName}
                      onChange={(event) => setAddUserName(event.target.value)}
                    />
                    <FormLabel htmlFor="amount">Password</FormLabel>
                    <Input
                      variant="flushed"
                      placeholder="User Name"
                      value={addUserName}
                      onChange={(event) => setAddUserName(event.target.value)}
                    />
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
          </Heading>
          {userList?.length === 0 ? (
            <Text fontSize="3xl">Nothing Here</Text>
          ) : (
            ""
          )}
          <TableContainer w={"100vw"}>
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
                      <Td>{user.userName}</Td>
                      <Td>
                        <Button
                          leftIcon={<GrUpdate />}
                          onClick={() => {
                            setEditUserName(user.userName);
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
                Delete API
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
            <ModalHeader>Edit User</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <Input
                variant="flushed"
                placeholder="User New Name"
                value={editUserName}
                onChange={(event) => setEditUserName(event.target.value)}
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
                  editUser(editUserId);
                  onCloseEditModal();
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
