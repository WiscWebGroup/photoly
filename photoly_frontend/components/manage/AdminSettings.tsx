import {
  Center,
  HStack,
  VStack,
  Box,
  Image,
  Alert,
  AlertIcon,
  Divider,
  Heading,
  Stack,
  Text,
  FormControl,
  FormLabel,
  SimpleGrid,
  Switch,
  Button,
  Select,
***REMOVED*** from "@chakra-ui/react";
import React, { useEffect, useState, useRef ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";

const AdminSettings: React.FC = () => {
  const [mySQLStatus, setMySQLStatus] = React.useState<string>("");
  const [redisStatus, setRedisStatus] = React.useState<string>("");

  const [parameterRes, setParameterRes] = React.useState<Map<string,string>>();

  return (
    <>
      <Center h="calc(100%-4rem)" w={"85vw"***REMOVED***>
        <VStack
          shadow={"lg"***REMOVED***
          w={"55%"***REMOVED***
          rounded={"lg"***REMOVED***
          m={8***REMOVED***
          p={8***REMOVED***
          bg={"white"***REMOVED***
          minH={"80vh"***REMOVED***
        >
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"***REMOVED***
            alignSelf={"flex-start"***REMOVED***
            p={3***REMOVED***
          >
            Service Status
          </Heading>

          <HStack spacing="24px" alignSelf={"flex-start"***REMOVED***>
            <Box
              w="20vw"
              h="13vh"
              bgGradient="linear(to-r, blue.50, orange.400)"
              alignSelf={"flex-start"***REMOVED***
              borderRadius={20***REMOVED***
            >
              <HStack spacing="24px" h="100%">
                <Box w="8vw" h={"100%"***REMOVED*** bg="gray.50" borderRadius={20***REMOVED***>
                  <Image src="/MySQL_ConnectorLogo.png" />
                </Box>
                <Box w="8vw" h="50%" borderRadius={20***REMOVED***>
                  <Alert status="success" h="100%" borderRadius={20***REMOVED***>
                    <AlertIcon />
                    Live
                  </Alert>
                </Box>
              </HStack>
            </Box>
            <Box
              w="20vw"
              h="13vh"
              bgGradient="linear(to-r, green.50, red.400)"
              alignSelf={"flex-start"***REMOVED***
              borderRadius={20***REMOVED***
            >
              <HStack spacing="24px" h="100%">
                <Box w="8vw" h={"100%"***REMOVED*** bg="gray.50" borderRadius={20***REMOVED***>
                  <Image src="/Redis_Logo.svg.png" />
                </Box>
                <Box w="8vw" h="50%" borderRadius={20***REMOVED***>
                  <Alert status="success" h="100%" borderRadius={20***REMOVED***>
                    <AlertIcon />
                    Live
                  </Alert>
                </Box>
              </HStack>
            </Box>
          </HStack>
          <Divider style={{ marginTop: "2rem" ***REMOVED******REMOVED*** />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"***REMOVED***
            alignSelf={"flex-start"***REMOVED***
            p={3***REMOVED***
          >
            Locations
          </Heading>
          <Stack
            direction={"row"***REMOVED***
            alignSelf="flex-start"
            spacing={"5"***REMOVED***
            w={"100%"***REMOVED***
          >
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">MySQL Addr</Heading>
              <Text mt={4***REMOVED***>192.168.1.1:3306</Text>
            </Box>
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">Redis Addr</Heading>
              <Text mt={4***REMOVED***>192.168.1.1:3679</Text>
            </Box>
          </Stack>
          <Stack
            direction={"row"***REMOVED***
            alignSelf="flex-start"
            spacing={"5"***REMOVED***
            style={{ marginTop: "1rem" ***REMOVED******REMOVED***
            w={"100%"***REMOVED***
          >
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">Upload Addr</Heading>
              <Text mt={4***REMOVED***>/www/wwwroot/photoly</Text>
            </Box>
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">Config Addr</Heading>
              <Text mt={4***REMOVED***>/www/wwwroot/photoly/photoly.config</Text>
            </Box>
          </Stack>
          <Divider style={{ marginTop: "2rem" ***REMOVED******REMOVED*** />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"***REMOVED***
            alignSelf={"flex-start"***REMOVED***
            p={3***REMOVED***
          >
            Settings
          </Heading>
          <Stack
            direction={"row"***REMOVED***
            alignSelf="flex-start"
            spacing={"5"***REMOVED***
            w={"100%"***REMOVED***
          >
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">General</Heading>
              <FormControl
                as={SimpleGrid***REMOVED***
                columns={{ base: 2, lg: 2 ***REMOVED******REMOVED***
                style={{ marginTop: "1rem" ***REMOVED******REMOVED***
              >
                <FormLabel htmlFor="isChecked">Allow User Register:</FormLabel>
                <Switch id="isChecked" isChecked size="md" />
                <FormLabel htmlFor="isChecked">S-Safe UUID Mode:</FormLabel>
                <Switch id="isChecked" isChecked size="md" />
              </FormControl>
              <Divider />
              <FormControl>
                <FormLabel htmlFor="isChecked">
                  SignIn Token Duration:
                </FormLabel>
                <Select id="country" placeholder="Select Duration">
                  <option value="1">1 Day</option>
                  <option value="3">3 Day</option>
                  <option value="5">5 Day</option>
                  <option value="7">6 Day</option>
                  <option value="14">14 Day</option>
                  <option value="30">30 Day</option>
                  <option value="60">60 Day</option>
                  <option value="90">90 Day</option>
                  <option value="120">120 Day</option>
                </Select>
              </FormControl>
            </Box>
            <Box
              p={5***REMOVED***
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"***REMOVED***
              w={"50%"***REMOVED***
            >
              <Heading fontSize="xl">Power</Heading>
              <Button
                colorScheme="red"
                style={{ marginTop: "1rem" ***REMOVED******REMOVED***
                w={"100%"***REMOVED***
                variant="outline"
              >
                Stop Service
              </Button>
            </Box>
          </Stack>
        </VStack>
      </Center>
    </>
  );
***REMOVED***;

export default AdminSettings;
