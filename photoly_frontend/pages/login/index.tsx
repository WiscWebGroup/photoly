import {Button, Center, Input, InputGroup, InputRightElement, Text, useBoolean, VStack***REMOVED*** from "@chakra-ui/react";
import React, {ChangeEvent, useEffect, useState***REMOVED*** from "react";
import {useRouter***REMOVED*** from "next/router";
import useLocalStorage, {TOKEN_KEY***REMOVED*** from "../../hooks/useLocalStorage";
import useApi from "../../hooks/useApi";

const Login: React.FC = () => {
    const [show, setShow] = useBoolean(false)
    const [email, setEmail] = useState<string | undefined>()
    const [password, setPassword] = useState<string | undefined>()
    const handleEmailChange = (event: ChangeEvent<HTMLInputElement>) => {
        setEmail(event.currentTarget.value)
***REMOVED***
    const handlePasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
        setPassword(event.currentTarget.value)
***REMOVED***
    const router = useRouter()
    const {getLS, setLS***REMOVED*** = useLocalStorage(TOKEN_KEY)
    const {isLoading, post***REMOVED*** = useApi()
    const handleLogin = async () => {
        await post("/user/signIn", {
            email: email,
            password: password
***REMOVED***, {***REMOVED***).then(res => {
            if (!!res) {
                setLS(res.data.token)
                router.push("/home")
    ***REMOVED***
***REMOVED***)
***REMOVED***
    // If the user has logged in, redirect to home
    useEffect(() => {
        const prev_token = getLS()
        if (prev_token !== null) {
            router.push("/home")
***REMOVED***
***REMOVED***, [getLS, router])


    return (
        <Center h="100vh">
            <VStack rounded="2xl" boxShadow="2xl" padding={8***REMOVED*** spacing={4***REMOVED*** w={{base: "80vw", md: "30vw"***REMOVED******REMOVED***
                    justify="space-evenly">
                <Text fontSize="3xl" color="gray.700">Login to Photoly</Text>
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

                <Button colorScheme="teal" w="100%" onClick={handleLogin***REMOVED*** isLoading={isLoading***REMOVED***
                        loadingText={"Submitting"***REMOVED***>Login</Button>
                <Button w="100%" onClick={() => router.push("/signup")***REMOVED*** isDisabled={isLoading***REMOVED***>Register</Button>
            </VStack>
        </Center>
    )
***REMOVED***
export default Login