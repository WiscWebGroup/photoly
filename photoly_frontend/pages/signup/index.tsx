import {Button, Center, Input, InputGroup, InputRightElement, Text, useBoolean, VStack} from "@chakra-ui/react";
import React from "react";
import useLocalStorage, {TOKEN_KEY} from "../../hooks/useLocalStorage";
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
        router
    ] = useLoginForm()
    const [show, setShow] = useBoolean(false)
    const {setLS} = useLocalStorage(TOKEN_KEY)
    const {post, isLoading} = useApi()

    const submitSignUp = async () => {
        await post("/user/signUp", {
            userName: username,
            email: email,
            password: password
        }, {}).then(res => {
            if (!!res && res.data.msgCode == 200) {
                setLS(res.data.token)
                router.push("/home")
            }
        })
    }


    return (
        <>
            <Center h="100vh">
                <VStack rounded="2xl" boxShadow="2xl" padding={8} spacing={4} w={{base: "80vw", md: "30vw"}}
                        justify="space-evenly">
                    <Text fontSize="3xl" color="gray.700">Sign Up to Photoly</Text>
                    <Input variant="outline" placeholder="Enter username" onChange={handleUsernameChange}
                           isDisabled={isLoading}></Input>
                    <Input variant="outline" placeholder="Enter email" onChange={handleEmailChange}
                           isDisabled={isLoading}></Input>
                    <InputGroup>
                        <Input variant="outline" type={show ? 'text' : 'password'} placeholder="Enter password"
                               onChange={handlePasswordChange} isDisabled={isLoading}></Input>
                        <InputRightElement pr={1}>
                            <Button padding={4} size='sm' onClick={setShow.toggle} fontSize="xs">
                                {show ? 'Hide' : 'Show'}
                            </Button>
                        </InputRightElement>
                    </InputGroup>

                    <Button colorScheme="teal" w="100%" onClick={submitSignUp} isLoading={isLoading}
                            loadingText={"Submitting"}>Register</Button>
                    <Button variant={"link"} onClick={() => router.push("/login")}>Already have an account?
                        Login</Button>
                </VStack>
            </Center>
        </>

    )
}
export default Login