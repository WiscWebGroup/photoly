import {Button, Center, Input, InputGroup, InputRightElement, Text, useBoolean, VStack} from "@chakra-ui/react";
import React, {ChangeEvent, useState} from "react";
import {useRouter} from "next/router";

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


    return (
        <Center h="100vh">
            <VStack rounded="2xl" boxShadow="2xl" padding={8} spacing={4} w={{base: "80vw", md: "30vw"}}
                    justify="space-evenly">
                <Text fontSize="3xl" color="gray.700">Login to Photoly</Text>
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

                <Button colorScheme="teal" w="100%">Login</Button>
                <Button w="100%" onClick={() => router.push("/signup")}>Register</Button>
            </VStack>
        </Center>
    )
}
export default Login