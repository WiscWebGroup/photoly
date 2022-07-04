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
import React, {ChangeEvent, useState} from "react";
import {useRouter} from "next/router";
import axios from "axios";
import useLocalStorage, {TOKEN_KEY} from "../../hooks/useLocalStorage";

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
    const {set} = useLocalStorage(TOKEN_KEY)

    const submitSignUp = () => {
        axios.post("/user/signUp", {
            userName: username,
            email: email,
            password: password
        }).then(res => {
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
                    <Input variant="outline" placeholder="Enter username" onChange={handleUsernameChange}></Input>
                    <Input variant="outline" placeholder="Enter email" onChange={handleEmailChange}></Input>
                    <InputGroup>
                        <Input variant="outline" type={show ? 'text' : 'password'} placeholder="Enter password"
                               onChange={handlePasswordChange}></Input>
                        <InputRightElement pr={1}>
                            <Button padding={4} size='sm' onClick={setShow.toggle} fontSize="xs">
                                {show ? 'Hide' : 'Show'}
                            </Button>
                        </InputRightElement>
                    </InputGroup>

                    <Button colorScheme="teal" w="100%" onClick={submitSignUp}>Register</Button>
                    <Button variant={"link"} onClick={() => router.push("/login")}>Already have an account?
                        Login</Button>
                </VStack>
            </Center>
        </>

    )
}
export default Login