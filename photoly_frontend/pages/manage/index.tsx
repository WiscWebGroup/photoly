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
import {AiOutlineEdit***REMOVED*** from "react-icons/ai";
import React, {useEffect, useState***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import ChangeInfoDrawer from "../../components/ChangeInfoDrawer";
import Navbar from "../../components/Navbar";
import {AiOutlineHome***REMOVED*** from "react-icons/ai";
import {FiSettings***REMOVED*** from "react-icons/fi";
import {RiFileSettingsLine***REMOVED*** from "react-icons/ri";

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
    const {get***REMOVED*** = useApi();
    const [info, setInfo] = useState<userInfo>();
    const {isOpen, onOpen, onClose***REMOVED*** = useDisclosure();

    const getInfo = async () => {
        const response = await get("/user/getInfo", {
            headers: {"HRD-token": token***REMOVED***,
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
        <>
            <Navbar/>

            <Box bg={"gray.50"***REMOVED*** h={"calc(100vh - 4rem)"***REMOVED***>
                <Slide
                    direction="left"
                    in={true***REMOVED***
                    style={{
                        height: "100%",
                        width: "150px",
                        zIndex: 200,
                        alignSelf: "flex-start",
                        background: "#FFFFFF",
                        marginTop: "4rem"
  ***REMOVED*****REMOVED*****REMOVED******REMOVED***
                >
                    <VStack spacing={0***REMOVED*** bg={"white"***REMOVED*** h={"100%"***REMOVED*** w={"max-content"***REMOVED***>
                        <Button
                            leftIcon={<AiOutlineHome/>***REMOVED***
                            colorScheme="teal"
                            variant="ghost"
                            padding={6***REMOVED***
                            minW="100%"
                            justifyContent="flex-start"
                        >
                            My Profile
                        </Button>
                        <Button
                            leftIcon={<FiSettings/>***REMOVED***
                            colorScheme="teal"
                            variant="ghost"
                            padding={6***REMOVED***
                            minW="100%"
                            justifyContent="flex-start"
                        >
                            Settings
                        </Button>
                        <Button
                            leftIcon={<RiFileSettingsLine/>***REMOVED***
                            colorScheme="teal"
                            variant="ghost"
                            padding={6***REMOVED***
                            minW="100%"
                            justifyContent="flex-start"
                        >
                            Admin Settings
                        </Button>
                    </VStack>
                </Slide>

                <ChangeInfoDrawer isOpen={isOpen***REMOVED*** onClose={onClose***REMOVED***/>
                <Center h={"100%"***REMOVED***>
                    <VStack
                        shadow={"lg"***REMOVED***
                        w={"45%"***REMOVED***
                        rounded={"lg"***REMOVED***
                        m={8***REMOVED***
                        p={8***REMOVED***
                        bg={"white"***REMOVED***
                    >
                        <HStack
                            justifyContent={"space-evenly"***REMOVED***
                            w={"100%"***REMOVED***
                            h="25vh"
                            style={{
                                backgroundImage: "linear-gradient(to right, #B2F5EA, #BEE3F8)",
***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                        >
                            <VStack>
                                <Avatar
                                    borderRadius="full"
                                    h={24***REMOVED***
                                    w={24***REMOVED***
                                    src={`user/getAvatar/${token***REMOVED***`***REMOVED***
                                />
                                <Text fontWeight={"semibold"***REMOVED*** fontSize={"xl"***REMOVED*** color={"#97266D"***REMOVED***>
                 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***info?.userName***REMOVED***
                                </Text>
                            </VStack>
                        </HStack>

                        <Divider/>
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
                            <GridItem colSpan={2***REMOVED*** padding={3***REMOVED*** bg={"orange.200"***REMOVED***>
                                <Heading
                                    as="h6"
                                    size="md"
                                    fontWeight={"450"***REMOVED***
                                    color="RGBA(0, 0, 0, 0.80)"
                                >
                                    Name
                                </Heading>
                                <Text marginTop={2***REMOVED*** marginLeft={1***REMOVED***>
                 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***info?.userName***REMOVED***
                                </Text>
                            </GridItem>
                            <GridItem colSpan={2***REMOVED*** padding={3***REMOVED*** bg="orange.200">
                                <Heading
                                    as="h6"
                                    size="md"
                                    fontWeight={"450"***REMOVED***
                                    color="RGBA(0, 0, 0, 0.80)"
                                >
                                    Register Time
                                </Heading>
                                <Text marginTop={2***REMOVED*** marginLeft={1***REMOVED***>
                 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***info?.createDate.substr(0, 10)***REMOVED***
                                </Text>
                            </GridItem>
                            <GridItem rowSpan={2***REMOVED*** padding={3***REMOVED*** colSpan={1***REMOVED*** bg={"purple.50"***REMOVED***>
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
                                    icon={<AiOutlineEdit/>***REMOVED***
                                    onClick={onOpen***REMOVED***
                                    size={"lg"***REMOVED***
                                    mt={4***REMOVED***
                                    padding={5***REMOVED***
                                    w={"100%"***REMOVED***
                                />
                            </GridItem>
                            <GridItem colSpan={4***REMOVED*** padding={3***REMOVED*** bg={"teal.100"***REMOVED***>
                                <Heading as="h6" size="md" fontWeight={"450"***REMOVED***>
                                    Email
                                </Heading>
                                <Text marginTop={2***REMOVED*** marginLeft={1***REMOVED***>
                 ***REMOVED*****REMOVED*****REMOVED*****REMOVED***info?.email***REMOVED***
                                </Text>
                            </GridItem>
                        </Grid>

                        <Divider/>
                        <Heading
                            as="h6"
                            size="md"
                            fontWeight={"450"***REMOVED***
                            alignSelf={"flex-start"***REMOVED***
                        >
                            API
                        </Heading>
                    </VStack>
                </Center></Box>
        </>

    );
***REMOVED***;
export default Manage;
