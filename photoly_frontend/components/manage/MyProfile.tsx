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
} from "@chakra-ui/react";
import { AiOutlineEdit } from "react-icons/ai";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import ChangeInfoDrawer from "../ChangeInfoDrawer";
import { AiOutlineCopy, AiOutlineDelete } from "react-icons/ai";
import { GrUpdate, GrAdd } from "react-icons/gr";
import { useRouter } from "next/router";
import useLocalStorage, {TOKEN_KEY} from "../../hooks/useLocalStorage";


interface userInfo {
  userId: number;
  userName: string;
  email: string;
  createDate: string;
  role: string;
  uuid: string;
}

interface cred {
  credId: number;
  userId: number;
  token: string;
  authorization: string;
}

const MyProfile: React.FC = () => {
  const token = useToken();
  const { get, post } = useApi();
  const [info, setInfo] = useState<userInfo>();
  const { isOpen, onOpen, onClose } = useDisclosure();

  const [credList, setCredList] = useState<cred[]>();
  const { value, setValue } = useCheckboxGroup({});

  const [delCredId, setDelCredId] = useState<number>();

  const router = useRouter()
  const {removeLS} = useLocalStorage(TOKEN_KEY);
  const toast = useToast();
  const {
    isOpen: isOpenDeleteConfirm,
    onOpen: onOpenDeleteConfirm,
    onClose: onCloseDeleteConfirm,
  } = useDisclosure();
  const cancelRef = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn = useRef<HTMLButtonElement>(null);

  const {
    isOpen: isOpenDeleteConfirm2,
    onOpen: onOpenDeleteConfirm2,
    onClose: onCloseDeleteConfirm2,
  } = useDisclosure();
  const cancelRef2 = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn2 = useRef<HTMLButtonElement>(null);

  const {
    isOpen: isOpenAdd,
    onOpen: onOpenAdd,
    onClose: onCloseAdd,
  } = useDisclosure();

  const getInfo = async () => {
    const response = await get("/user/getInfo", {
      headers: { "HRD-token": token },
    });
    if (!!response && response.data.msgCode === 200) {
      setInfo(response.data.t);
    }
  };

  const getCred = async () => {
    const response = await get("/cred/query", {
      headers: { "HRD-token": token },
    });
    if (!!response && response.data.msgCode === 200) {
      setCredList(response.data.t);
    }
  };

  const delCred = async () => {
    const response = await post(
      "/cred/delete",
      {},
      {
        headers: { "HRD-token": token },
        params: {
          credId: delCredId,
        },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Delete Successful`,
        status: "success",
        isClosable: true,
        position: "top",
        duration: 3000,
      });
      getCred();
    }
  };

  const getEditCred = (auth: string) => {
    var lst = [];
    if (auth.includes("C")) {
      lst?.push("C");
    }
    if (auth.includes("R")) {
      lst?.push("R");
    }
    if (auth.includes("D")) {
      lst?.push("D");
    }
    setValue(lst);
  };

  const editCred = async (credId: number) => {
    const response = await post(
      "/cred/update",
      {
        credId: credId,
        authorization: value.join(),
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
      getCred();
    }
  };

  const addCred = async () => {
    const response = await post(
      "/cred/create",
      {
        authorization: value.join(),
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
      getCred();
    }
  };

  const copyToken = (token: string) => {
    navigator.clipboard.writeText(token);
    toast({
      title: `Copied to your Clipboard`,
      status: "success",
      isClosable: true,
      duration: 3000,
      position: "top",
    });
  };

  const delUser = async () => {
    const response = await get(
      "/user/deleteUser",
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Delete Successful`,
        status: "success",
        isClosable: true,
        position: "top",
        duration: 3000,
      });
    }
    router.push("/login")
    removeLS()
  };

  useEffect(() => {
    if (!!token) {
      getInfo();
      getCred();
    }
  }, [token]);

  return (
    <>
      <ChangeInfoDrawer isOpen={isOpen} onClose={onClose} />
      <Center h="calc(100%-4rem)" w={"85vw"}>
        <VStack shadow={"lg"} w={"55%"} rounded={"lg"} m={8} p={8} bg={"white"}>
          <HStack
            justifyContent={"space-evenly"}
            w={"100%"}
            h="25vh"
            style={{
              backgroundImage: "linear-gradient(to right, #B2F5EA, #BEE3F8)",
            }}
            rounded={"lg"}
          >
            <VStack>
              <Avatar
                borderRadius="full"
                h={24}
                w={24}
                src={`user/getAvatar/${token}`}
              />
              <Text fontWeight={"semibold"} fontSize={"xl"} color={"#97266D"}>
                {info?.userName}
              </Text>
            </VStack>
          </HStack>

          <Divider style={{ marginTop: "2rem" }} />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
          >
            Personal Profile
          </Heading>
          <Grid
            h="200px"
            templateRows="repeat(2, 1fr)"
            templateColumns="repeat(5, 1fr)"
            gap={4}
            width="100%"
          >
            <GridItem colSpan={2} padding={3} bg={"orange.200"} rounded={"lg"}>
              <Heading
                as="h6"
                size="md"
                fontWeight={"450"}
                color="RGBA(0, 0, 0, 0.80)"
              >
                Name
              </Heading>
              <Text marginTop={2} marginLeft={1}>
                {info?.userName}
              </Text>
            </GridItem>
            <GridItem colSpan={2} padding={3} bg="orange.200" rounded={"lg"}>
              <Heading
                as="h6"
                size="md"
                fontWeight={"450"}
                color="RGBA(0, 0, 0, 0.80)"
              >
                Register Time
              </Heading>
              <Text marginTop={2} marginLeft={1}>
                {info?.createDate.substr(0, 10)}
              </Text>
            </GridItem>
            <GridItem
              rowSpan={2}
              padding={3}
              colSpan={1}
              bg={"purple.50"}
              rounded={"lg"}
            >
              <Heading
                as="h6"
                size="md"
                fontWeight={"450"}
                color="RGBA(0, 0, 0, 0.80)"
              >
                Edit Profile
              </Heading>
              <IconButton
                variant={"ghost"}
                aria-label={"Edit profile"}
                icon={<AiOutlineEdit />}
                onClick={onOpen}
                size={"lg"}
                mt={4}
                padding={5}
                w={"100%"}
              />
            </GridItem>
            <GridItem colSpan={4} padding={3} bg={"teal.100"} rounded={"lg"}>
              <Heading as="h6" size="md" fontWeight={"450"}>
                Email
              </Heading>
              <Text marginTop={2} marginLeft={1}>
                {info?.email}
              </Text>
            </GridItem>
          </Grid>

          <Divider style={{ marginTop: "2rem" }} />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
          >
            API
            <Popover
              isOpen={isOpenAdd}
              onClose={() => {
                onCloseAdd();
              }}
            >
              <PopoverTrigger>
                <Button
                  colorScheme="teal"
                  variant="ghost"
                  ml={4}
                  rightIcon={<GrAdd />}
                  onClick={() => {
                    onOpenAdd();
                    setValue([]);
                  }}
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
                    defaultValue={[]}
                    value={value}
                    onChange={(e) => {
                      setValue(e);
                    }}
                  >
                    <Stack spacing={[1, 5]} direction={["column", "row"]}>
                      <Checkbox value="C">Upload</Checkbox>
                      <Checkbox value="R">Read</Checkbox>
                      <Checkbox value="D">Delete</Checkbox>
                    </Stack>
                  </CheckboxGroup>
                  <Button
                    colorScheme="twitter"
                    variant="outline"
                    width={"100%"}
                    mt={5}
                    onClick={() => {
                      addCred();
                      onCloseAdd();
                    }}
                  >
                    Create
                  </Button>
                </PopoverBody>
              </PopoverContent>
            </Popover>
          </Heading>
          <Stack width={"100%"}>
            {credList?.length === 0 ? (
              <>
                <Text fontSize="xl">Nothing Here</Text>
                <Divider />
              </>
            ) : (
              <TableContainer w={"100vw"}>
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
                    {credList?.map((cred) => {
                      return (
                        <Tr key={cred.credId}>
                          <Td>{cred.credId}</Td>
                          <Td>
                            {cred.authorization
                              .replace("C", "Upload ")
                              .replace("R", "Read ")
                              .replace("D", "Delete")}
                          </Td>
                          <Td>{cred.token}</Td>
                          <Td>
                            <Button
                              leftIcon={<AiOutlineCopy />}
                              onClick={() => {
                                copyToken(cred.token);
                              }}
                            ></Button>
                            <Popover>
                              <PopoverTrigger>
                                <Button
                                  leftIcon={<GrUpdate />}
                                  onClick={() => {
                                    getEditCred(cred.authorization);
                                  }}
                                ></Button>
                              </PopoverTrigger>
                              <PopoverContent>
                                <PopoverArrow />
                                <PopoverCloseButton />
                                <PopoverHeader>Update Auth</PopoverHeader>
                                <PopoverBody>
                                  <CheckboxGroup
                                    colorScheme="green"
                                    value={value}
                                    onChange={(e) => {
                                      setValue(e);
                                    }}
                                  >
                                    <Stack
                                      spacing={[1, 5]}
                                      direction={["column", "row"]}
                                    >
                                      <Checkbox value="C">Upload</Checkbox>
                                      <Checkbox value="R">Read</Checkbox>
                                      <Checkbox value="D">Delete</Checkbox>
                                    </Stack>
                                  </CheckboxGroup>
                                  <Button
                                    colorScheme="twitter"
                                    variant="outline"
                                    width={"100%"}
                                    mt={5}
                                    onClick={() => {
                                      editCred(cred.credId);
                                    }}
                                  >
                                    Confirm
                                  </Button>
                                </PopoverBody>
                              </PopoverContent>
                            </Popover>

                            <Button
                              leftIcon={<AiOutlineDelete />}
                              onClick={() => {
                                setDelCredId(cred.credId);
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
            )}

            <Heading
              as="h6"
              size="md"
              fontWeight={"450"}
              alignSelf={"flex-start"}
              style={{ marginTop: 20 }}
            >
              API Documents
            </Heading>
            <Accordion
              defaultIndex={[]}
              allowMultiple
              style={{ marginTop: 20 }}
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
                <AccordionPanel pb={4}>
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
                <AccordionPanel pb={4}>
                  <Text fontSize="xl">C1 Upload Photo - POST:</Text>
                  <br></br>
                  Addr: server_path/cred/uploadPhoto/YOUR_TOKEN
                  <br></br>
                  Takein: file:MultipartFile | photo:JSONText containing
                  nsId(num) and visibility(num)
                  <br></br>
                  Return: a string that is the photo&apos;s uuid
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
                  Takein: uuid: str, photo&apos;s uuid
                  <br></br>
                  Return: an int, 1 if successful
                  <br></br>
                  <br></br>
                  <Text fontSize="xl">R1 Render Photo - POST:</Text>
                  <br></br>
                  Addr: server_path/cred/render/YOUR_TOKEN
                  <br></br>
                  Takein: uuid: str, photo&apos;s uuid
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
                  Return: an JSON object that is the list of photos&apos; meta info
                  <br></br>
                  <br></br>
                </AccordionPanel>
              </AccordionItem>
            </Accordion>
            <Divider style={{ marginTop: "2rem" }} />
            <Heading
              as="h6"
              size="md"
              fontWeight={"450"}
              alignSelf={"flex-start"}
            >
              Other
            </Heading>
            <Button colorScheme="red" variant="ghost" w={"100%"} onClick={onOpenDeleteConfirm2}>
              Delete my Account
            </Button>
          </Stack>
        </VStack>
      </Center>
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
              Are you sure? You can&apos;t undo this action afterwards.
            </AlertDialogBody>

            <AlertDialogFooter>
              <Button ref={cancelRefBtn} onClick={onCloseDeleteConfirm}>
                Cancel
              </Button>
              <Button
                colorScheme="red"
                onClick={() => {
                  delCred();
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

      <AlertDialog
        isOpen={isOpenDeleteConfirm2}
        leastDestructiveRef={cancelRef2}
        onClose={onCloseDeleteConfirm2}
      >
        <AlertDialogOverlay>
          <AlertDialogContent>
            <AlertDialogHeader fontSize="lg" fontWeight="bold">
              DELETE USER ACCOUNT
            </AlertDialogHeader>

            <AlertDialogBody>
              Are you sure? You can&apos;t undo this action afterwards.
            </AlertDialogBody>

            <AlertDialogFooter>
              <Button ref={cancelRefBtn2} onClick={onCloseDeleteConfirm2}>
                Cancel
              </Button>
              <Button
                colorScheme="red"
                onClick={() => {
                  delUser();
                  onCloseDeleteConfirm2();
                }}
                ml={3}
              >
                DELETE
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
    </>
  );
};
export default MyProfile;
