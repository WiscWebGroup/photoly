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
} from "@chakra-ui/react";
import React from "react";
import axios from "axios";
import useLocalStorage, {TOKEN_KEY} from "../../hooks/useLocalStorage";
import useLoginForm from "../../hooks/useLoginForm";

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
    const toast = useToast()
    const {set} = useLocalStorage(TOKEN_KEY)

    const submitSignUp = async () => {
        setLoading.on()
        await axios.post("/user/signUp", {
            userName: username,
            email: email,
            password: password
        }).then(res => {
            setLoading.off()
            if (res.data.msgCode !== 200) {
                toast({title: res.data.t, status: "error", isClosable: true, position: "top"})
            } else {
                toast({title: res.data.t, status: "success", isClosable: true, position: "top"})
                set(res.data.token)
                router.push("/home")
            }
        }).catch(err => {
            toast({title: err, status: "error", isClosable: true, position: "top"})
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