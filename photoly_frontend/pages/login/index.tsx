import {Button, Center, Input, InputGroup, InputRightElement, Text, useBoolean, VStack} from "@chakra-ui/react";
import React, {ChangeEvent, useEffect, useState} from "react";
import {useRouter} from "next/router";
import useLocalStorage, {TOKEN_KEY} from "../../hooks/useLocalStorage";
import useApi from "../../hooks/useApi";

const Login: React.FC = () => {
    const [show, setShow] = useBoolean(false)
    const [email, setEmail] = useState<string | undefined>()
    const [password, setPassword] = useState<string | undefined>()
    const handleEmailChange = (event: ChangeEvent<HTMLInputElement>) => {
        setEmail(event.currentTarget.value)
    }
    const handlePasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
        setPassword(event.currentTarget.value)
    }
    const router = useRouter()
    const {getLS, setLS} = useLocalStorage(TOKEN_KEY)
    const {isLoading, post} = useApi()
    const handleLogin = async () => {
        await post("/user/signIn", {
            email: email,
            password: password
        }).then(res => {
            if (!!res) {
                setLS(res.data.token)
                router.push("/home")
            }
        })
    }
    // If the user has logged in, redirect to home
    useEffect(() => {
        if (getLS() !== null) {
            router.push("/home")
        }
    }, [getLS, router])


    return (
        <Center h="100vh">
            <VStack rounded="2xl" boxShadow="2xl" padding={8} spacing={4} w={{base: "80vw", md: "30vw"}}
                    justify="space-evenly">
                <Text fontSize="3xl" color="gray.700">Login to Photoly</Text>
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

                <Button colorScheme="teal" w="100%" onClick={handleLogin} isLoading={isLoading}
                        loadingText={"Submitting"}>Login</Button>
                <Button w="100%" onClick={() => router.push("/signup")} isDisabled={isLoading}>Register</Button>
            </VStack>
        </Center>
    )
}
export default Login