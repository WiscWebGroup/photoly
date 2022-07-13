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
***REMOVED*** from "@chakra-ui/react";
import { AiOutlineEdit ***REMOVED*** from "react-icons/ai";
import React, { useEffect, useState ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import ChangeInfoDrawer from "../../components/ChangeInfoDrawer";
import Navbar from "../../components/Navbar";
import { AiOutlineHome ***REMOVED*** from "react-icons/ai";
import { FiSettings ***REMOVED*** from "react-icons/fi";
import { RiFileSettingsLine ***REMOVED*** from "react-icons/ri";

interface userInfo {
  userId: number;
  userName: string;
  email: string;
  createDate: string;
  role: string;
  uuid: string;
***REMOVED***

const Manage: React.FC = () => {
  const token = useToken();
  const { get ***REMOVED*** = useApi();
  const [info, setInfo] = useState<userInfo>();
  const { isOpen, onOpen, onClose ***REMOVED*** = useDisclosure();

  const getInfo = async () => {
    const response = await get("/user/getInfo", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setInfo(response.data.t);
***REMOVED***
  ***REMOVED***;
  useEffect(() => {
    if (!!token) {
      getInfo();
***REMOVED***
  ***REMOVED***, [token]);

  return (
    <div style={{ backgroundColor: "RGBA(0, 0, 0, 0.02)", height: "100vh" ***REMOVED******REMOVED***>
      <Navbar />
      <Slide
        direction="left"
        in={true***REMOVED***
        style={{
          height: "100vh",
          width: "300px",
          zIndex: 200,
          marginTop: "7vh",
          borderRight: "2px solid rgba(214, 214, 214, .5)",
          backgroundColor: "#FFFFFF",
***REMOVED******REMOVED***
      >
        <VStack spacing={0***REMOVED*** align="stretch">
          <Box h="45px">
            <Button
              leftIcon={<AiOutlineHome />***REMOVED***
              colorScheme="teal"
              variant="ghost"
              padding={6***REMOVED***
              minW="100%"
              justifyContent="flex-start"
            >
              My Profile
            </Button>
          </Box>
          <Box h="45px">
            <Button
              leftIcon={<FiSettings />***REMOVED***
              colorScheme="teal"
              variant="ghost"
              padding={6***REMOVED***
              minW="100%"
              justifyContent="flex-start"
            >
              Settings
            </Button>
          </Box>
          <Box h="45px">
            <Button
              leftIcon={<RiFileSettingsLine />***REMOVED***
              colorScheme="teal"
              variant="ghost"
              padding={6***REMOVED***
              minW="100%"
              justifyContent="flex-start"
            >
              Admin Settings
            </Button>
          </Box>
        </VStack>
      </Slide>

      <ChangeInfoDrawer isOpen={isOpen***REMOVED*** onClose={onClose***REMOVED*** />
      <Center>
        <VStack
          shadow={"lg"***REMOVED***
          w={"45%"***REMOVED***
          rounded={"lg"***REMOVED***
          m={8***REMOVED***
          p={8***REMOVED***
          style={{ backgroundColor: "#FFFFFF" ***REMOVED******REMOVED***
        >
          <HStack
            justifyContent={"space-evenly"***REMOVED***
            w={"100%"***REMOVED***
            h="25vh"
            style={{
              backgroundImage: "linear-gradient(to right, #B2F5EA, #BEE3F8)",
    ***REMOVED******REMOVED***
          >
            <VStack>
              <Avatar
                borderRadius="full"
                h={24***REMOVED***
                w={24***REMOVED***
                src={`user/getAvatar/${token***REMOVED***`***REMOVED***
              />
              <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED*** color={"#97266D"***REMOVED***>
            ***REMOVED***info?.userName***REMOVED***
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
            <GridItem colSpan={2***REMOVED*** padding={3***REMOVED*** bg="papayawhip">
              <Heading
                as="h6"
                size="md"
                fontWeight={"450"***REMOVED***
                color="RGBA(0, 0, 0, 0.80)"
              >
                Name
              </Heading>
              <Text marginTop={2***REMOVED*** marginLeft={1***REMOVED***>
            ***REMOVED***info?.userName***REMOVED***
              </Text>
            </GridItem>
            <GridItem colSpan={2***REMOVED*** padding={3***REMOVED*** bg="papayawhip">
              <Heading
                as="h6"
                size="md"
                fontWeight={"450"***REMOVED***
                color="RGBA(0, 0, 0, 0.80)"
              >
                Register Time
              </Heading>
              <Text marginTop={2***REMOVED*** marginLeft={1***REMOVED***>
            ***REMOVED***info?.createDate.substr(0, 10)***REMOVED***
              </Text>
            </GridItem>
            <GridItem rowSpan={2***REMOVED*** padding={3***REMOVED*** colSpan={1***REMOVED*** bg="#FAF5FF">
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
                padding={5***REMOVED***
              />
            </GridItem>
            <GridItem colSpan={4***REMOVED*** padding={3***REMOVED*** bg="#B2F5EA">
              <Heading as="h6" size="md" fontWeight={"450"***REMOVED***>
                Email
              </Heading>
              <Text marginTop={2***REMOVED*** marginLeft={1***REMOVED***>
            ***REMOVED***info?.email***REMOVED***
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
          </Heading>
        </VStack>
      </Center>
    </div>
  );
***REMOVED***;
export default Manage;
