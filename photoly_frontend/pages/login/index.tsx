import {Button, Center, Input, InputGroup, InputRightElement, Text, useBoolean, VStack***REMOVED*** from "@chakra-ui/react";
import {useState***REMOVED*** from "react";
import {useRouter***REMOVED*** from "next/router";

export default function Login() {
    const [show, setShow] = useBoolean(false)
    const [email, setEmail] = useState()
    const [password, setPassword] = useState()
    const handleEmailChange = (event) => setEmail(event.target.value)
    const handlePasswordChange = (event) => setPassword(event.target.value)
    const router = useRouter()


    return (
        <Center h="100vh">
            <VStack rounded="2xl" boxShadow="2xl" padding={8***REMOVED*** spacing={4***REMOVED*** w={{base: "80vw", md: "30vw"***REMOVED******REMOVED***
                    justify="space-evenly">
                <Text fontSize="3xl" color="gray.700">Login to Photoly</Text>
                <Input variant="outline" placeholder="Enter email" onChange={handleEmailChange***REMOVED***></Input>
                <InputGroup>
                    <Input variant="outline" type={show ? 'text' : 'password'***REMOVED*** placeholder="Enter password"
                           onChange={handlePasswordChange***REMOVED***></Input>
                    <InputRightElement pr={1***REMOVED***>
                        <Button padding={4***REMOVED*** size='sm' onClick={setShow.toggle***REMOVED*** fontSize="xs">
    ***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED***show ? 'Hide' : 'Show'***REMOVED***
                        </Button>
                    </InputRightElement>
                </InputGroup>

                <Button colorScheme="teal" w="100%">Login</Button>
                <Button w="100%" onClick={() => router.push("/signup")***REMOVED***>Register</Button>
            </VStack>
        </Center>
    )
***REMOVED***
