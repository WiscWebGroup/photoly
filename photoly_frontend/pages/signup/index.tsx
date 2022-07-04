import {
    Button,
    Center,
    Input,
    InputGroup,
    InputRightElement,
    Text,
    useBoolean,
    useToast,
    VStack
***REMOVED*** from "@chakra-ui/react";
import React, {ChangeEvent, useState***REMOVED*** from "react";
import {useRouter***REMOVED*** from "next/router";
import axios from "axios";
import useLocalStorage, {TOKEN_KEY***REMOVED*** from "../../hooks/useLocalStorage";

const Login: React.FC = () => {
    const [show, setShow] = useBoolean(false)
    const [username, setUsername] = useState<string | undefined>()
    const [email, setEmail] = useState<string | undefined>()
    const [password, setPassword] = useState<string | undefined>()
    const handleUsernameChange = (event: ChangeEvent<HTMLInputElement>) => setUsername(event.currentTarget.value)
    const handleEmailChange = (event: ChangeEvent<HTMLInputElement>) => setEmail(event.currentTarget.value)
    const handlePasswordChange = (event: ChangeEvent<HTMLInputElement>) => setPassword(event.currentTarget.value)
    const router = useRouter()
    const toast = useToast()
    const {set***REMOVED*** = useLocalStorage(TOKEN_KEY)

    const submitSignUp = () => {
        axios.post("/user/signUp", {
            userName: username,
            email: email,
            password: password
***REMOVED***).then(res => {
            if (res.data.msgCode !== 200) {
                toast({title: res.data.t, status: "error", isClosable: true, position: "top"***REMOVED***)
    ***REMOVED*** else {
                toast({title: res.data.t, status: "success", isClosable: true, position: "top"***REMOVED***)
                set(res.data.token)
                router.push("/home")
    ***REMOVED***
***REMOVED***).catch(err => {
            toast({title: err, status: "error", isClosable: true, position: "top"***REMOVED***)
***REMOVED***)
***REMOVED***


    return (
        <>
            <Center h="100vh">
                <VStack rounded="2xl" boxShadow="2xl" padding={8***REMOVED*** spacing={4***REMOVED*** w={{base: "80vw", md: "30vw"***REMOVED******REMOVED***
                        justify="space-evenly">
                    <Text fontSize="3xl" color="gray.700">Sign Up to Photoly</Text>
                    <Input variant="outline" placeholder="Enter username" onChange={handleUsernameChange***REMOVED***></Input>
                    <Input variant="outline" placeholder="Enter email" onChange={handleEmailChange***REMOVED***></Input>
                    <InputGroup>
                        <Input variant="outline" type={show ? 'text' : 'password'***REMOVED*** placeholder="Enter password"
                               onChange={handlePasswordChange***REMOVED***></Input>
                        <InputRightElement pr={1***REMOVED***>
                            <Button padding={4***REMOVED*** size='sm' onClick={setShow.toggle***REMOVED*** fontSize="xs">
   ***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED***show ? 'Hide' : 'Show'***REMOVED***
                            </Button>
                        </InputRightElement>
                    </InputGroup>

                    <Button colorScheme="teal" w="100%" onClick={submitSignUp***REMOVED***>Register</Button>
                    <Button variant={"link"***REMOVED*** onClick={() => router.push("/login")***REMOVED***>Already have an account?
                        Login</Button>
                </VStack>
            </Center>
        </>

    )
***REMOVED***
export default Login