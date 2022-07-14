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
  Slide,
  Box,
  Button,
  Grid,
  GridItem,
  Table,
  Thead,
  Tbody,
  Tfoot,
  Tr,
  Th,
  Td,
  TableCaption,
  TableContainer,
  Stack,
} from "@chakra-ui/react";
import { AiOutlineEdit } from "react-icons/ai";
import React, { useEffect, useState } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import ChangeInfoDrawer from "../../components/ChangeInfoDrawer";
import Navbar from "../../components/Navbar";
import { AiOutlineHome, AiOutlineCopy, AiOutlineDelete } from "react-icons/ai";
import { FiSettings } from "react-icons/fi";
import { RiFileSettingsLine } from "react-icons/ri";
import { GrUpdate,GrAdd } from "react-icons/gr";


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

const Manage: React.FC = () => {
  const token = useToken();
  const { get } = useApi();
  const [info, setInfo] = useState<userInfo>();
  const { isOpen, onOpen, onClose } = useDisclosure();

  const [credList, setCredList] = useState<cred[]>();
  const [selectedAuth, setSelectedAuth] = useState<string>();

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

  useEffect(() => {
    if (!!token) {
      getInfo();
      getCred();
    }
  }, [token]);

  return (
    <>
      <Navbar />

      <Stack bg={"gray.50"} h={"calc(100% - 4rem)"} direction="row" w="100vw-4rem">
      <VStack spacing={0} bg={"white"} h={"calc(100%-4rem)"} w={"15vw"}>
            <Button
              leftIcon={<AiOutlineHome />}
              colorScheme="teal"
              variant="ghost"
              padding={6}
              minW="100%"
              justifyContent="flex-start"
            >
              My Profile
            </Button>
            <Button
              leftIcon={<FiSettings />}
              colorScheme="teal"
              variant="ghost"
              padding={6}
              minW="100%"
              justifyContent="flex-start"
            >
              Settings
            </Button>
            <Button
              leftIcon={<RiFileSettingsLine />}
              colorScheme="teal"
              variant="ghost"
              padding={6}
              minW="100%"
              justifyContent="flex-start"
            >
              Admin Settings
            </Button>
          </VStack>
        
        <ChangeInfoDrawer isOpen={isOpen} onClose={onClose} />
        <Center h="calc(100%-4rem)" w={"85vw"}>
          <VStack
            shadow={"lg"}
            w={"55%"}
            rounded={"lg"}
            m={8}
            p={8}
            bg={"white"}
          >
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

            <Divider />
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
              <GridItem
                colSpan={2}
                padding={3}
                bg={"orange.200"}
                rounded={"lg"}
              >
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

            <Divider />
            <Heading
              as="h6"
              size="md"
              fontWeight={"450"}
              alignSelf={"flex-start"}
            >
              API
              <Button colorScheme='teal' variant='ghost' ml={4} rightIcon={<GrAdd/>}>
                New API
              </Button>
            </Heading>
            <Stack width={"100%"}>
            <TableContainer w={"100vw"}>
              <Table variant='simple'>
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
                    return <Tr key={cred.credId}>
                    <Td>{cred.credId}</Td>
                    <Td>{cred.authorization.replace("C", "Upload ").replace("R", "Read ").replace("D", "Delete")}</Td>
                    <Td>{cred.token}</Td>
                    <Td><Button leftIcon={<AiOutlineCopy/>}></Button><Button leftIcon={<GrUpdate/>}></Button><Button leftIcon={<AiOutlineDelete/>}></Button></Td>
                  </Tr>;
                  })}
                </Tbody>
              </Table>
            </TableContainer>
            </Stack>
          </VStack>
        </Center>
      </Stack>
    </>
  );
};
export default Manage;
