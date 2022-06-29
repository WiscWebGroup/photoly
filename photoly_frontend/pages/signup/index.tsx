import {Button, Center, Input, InputGroup, InputRightElement, Text, useBoolean, VStack***REMOVED*** from "@chakra-ui/react";
import {useState***REMOVED*** from "react";
import {useRouter***REMOVED*** from "next/router";

export default function Login() {
    const [show, setShow] = useBoolean(false)
    const [username, setUsername] = useState()
    const [email, setEmail] = useState()
    const [password, setPassword] = useState()
    const handleUsernameChange = (event) => setUsername(event.target.value)
    const handleEmailChange = (event) => setEmail(event.target.value)
    const handlePasswordChange = (event) => setPassword(event.target.value)
    const router = useRouter()


    return (
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
                        ***REMOVED***show ? 'Hide' : 'Show'***REMOVED***
                        </Button>
                    </InputRightElement>
                </InputGroup>

                <Button colorScheme="teal" w="100%">Register</Button>
                <Button variant={"link"***REMOVED*** onClick={() => router.push("/login")***REMOVED***>Already have an account? Login</Button>
            </VStack>
        </Center>
    )
***REMOVED***