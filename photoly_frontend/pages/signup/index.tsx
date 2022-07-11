import {Button, Center, Input, InputGroup, InputRightElement, Text, useBoolean, VStack***REMOVED*** from "@chakra-ui/react";
import React from "react";
import useLocalStorage, {TOKEN_KEY***REMOVED*** from "../../hooks/useLocalStorage";
import useLoginForm from "../../hooks/useLoginForm";
import useApi from "../../hooks/useApi";

const Login: React.FC = () => {
    const [
        username,
        email,
        password,
        handleUsernameChange,
        handleEmailChange,
        handlePasswordChange,
        router,
        isLoading,
        setLoading
    ] = useLoginForm()
    const [show, setShow] = useBoolean(false)
    const {set***REMOVED*** = useLocalStorage(TOKEN_KEY)
    const {post***REMOVED*** = useApi()

    const submitSignUp = async () => {
        setLoading.on()
        await post("/user/signUp", {
            userName: username,
            email: email,
            password: password
***REMOVED***).then(res => {
            setLoading.off()
            set(res.data.token)
***REMOVED***)
***REMOVED***


    return (
        <>
            <Center h="100vh">
                <VStack rounded="2xl" boxShadow="2xl" padding={8***REMOVED*** spacing={4***REMOVED*** w={{base: "80vw", md: "30vw"***REMOVED******REMOVED***
                        justify="space-evenly">
                    <Text fontSize="3xl" color="gray.700">Sign Up to Photoly</Text>
                    <Input variant="outline" placeholder="Enter username" onChange={handleUsernameChange***REMOVED***
                           isDisabled={isLoading***REMOVED***></Input>
                    <Input variant="outline" placeholder="Enter email" onChange={handleEmailChange***REMOVED***
                           isDisabled={isLoading***REMOVED***></Input>
                    <InputGroup>
                        <Input variant="outline" type={show ? 'text' : 'password'***REMOVED*** placeholder="Enter password"
                               onChange={handlePasswordChange***REMOVED*** isDisabled={isLoading***REMOVED***></Input>
                        <InputRightElement pr={1***REMOVED***>
                            <Button padding={4***REMOVED*** size='sm' onClick={setShow.toggle***REMOVED*** fontSize="xs">
                            ***REMOVED***show ? 'Hide' : 'Show'***REMOVED***
                            </Button>
                        </InputRightElement>
                    </InputGroup>

                    <Button colorScheme="teal" w="100%" onClick={submitSignUp***REMOVED*** isLoading={isLoading***REMOVED***
                            loadingText={"Submitting"***REMOVED***>Register</Button>
                    <Button variant={"link"***REMOVED*** onClick={() => router.push("/login")***REMOVED***>Already have an account?
                        Login</Button>
                </VStack>
            </Center>
        </>

    )
***REMOVED***
export default Login