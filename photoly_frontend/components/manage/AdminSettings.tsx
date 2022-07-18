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
} from "@chakra-ui/react";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";

const AdminSettings: React.FC = () => {
  const [mySQLStatus, setMySQLStatus] = React.useState<string>("");
  const [redisStatus, setRedisStatus] = React.useState<string>("");

  const [parameterRes, setParameterRes] = React.useState<Map<string,string>>();

  return (
    <>
      <Center h="calc(100%-4rem)" w={"85vw"}>
        <VStack
          shadow={"lg"}
          w={"55%"}
          rounded={"lg"}
          m={8}
          p={8}
          bg={"white"}
          minH={"80vh"}
        >
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
            p={3}
          >
            Service Status
          </Heading>

          <HStack spacing="24px" alignSelf={"flex-start"}>
            <Box
              w="20vw"
              h="13vh"
              bgGradient="linear(to-r, blue.50, orange.400)"
              alignSelf={"flex-start"}
              borderRadius={20}
            >
              <HStack spacing="24px" h="100%">
                <Box w="8vw" h={"100%"} bg="gray.50" borderRadius={20}>
                  <Image src="/MySQL_ConnectorLogo.png" />
                </Box>
                <Box w="8vw" h="50%" borderRadius={20}>
                  <Alert status="success" h="100%" borderRadius={20}>
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
              alignSelf={"flex-start"}
              borderRadius={20}
            >
              <HStack spacing="24px" h="100%">
                <Box w="8vw" h={"100%"} bg="gray.50" borderRadius={20}>
                  <Image src="/Redis_Logo.svg.png" />
                </Box>
                <Box w="8vw" h="50%" borderRadius={20}>
                  <Alert status="success" h="100%" borderRadius={20}>
                    <AlertIcon />
                    Live
                  </Alert>
                </Box>
              </HStack>
            </Box>
          </HStack>
          <Divider style={{ marginTop: "2rem" }} />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
            p={3}
          >
            Locations
          </Heading>
          <Stack
            direction={"row"}
            alignSelf="flex-start"
            spacing={"5"}
            w={"100%"}
          >
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">MySQL Addr</Heading>
              <Text mt={4}>192.168.1.1:3306</Text>
            </Box>
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">Redis Addr</Heading>
              <Text mt={4}>192.168.1.1:3679</Text>
            </Box>
          </Stack>
          <Stack
            direction={"row"}
            alignSelf="flex-start"
            spacing={"5"}
            style={{ marginTop: "1rem" }}
            w={"100%"}
          >
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">Upload Addr</Heading>
              <Text mt={4}>/www/wwwroot/photoly</Text>
            </Box>
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">Config Addr</Heading>
              <Text mt={4}>/www/wwwroot/photoly/photoly.config</Text>
            </Box>
          </Stack>
          <Divider style={{ marginTop: "2rem" }} />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
            p={3}
          >
            Settings
          </Heading>
          <Stack
            direction={"row"}
            alignSelf="flex-start"
            spacing={"5"}
            w={"100%"}
          >
            <Box
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">General</Heading>
              <FormControl
                as={SimpleGrid}
                columns={{ base: 2, lg: 2 }}
                style={{ marginTop: "1rem" }}
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
              p={5}
              shadow="md"
              borderWidth="1px"
              borderRadius={"20"}
              w={"50%"}
            >
              <Heading fontSize="xl">Power</Heading>
              <Button
                colorScheme="red"
                style={{ marginTop: "1rem" }}
                w={"100%"}
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
};

export default AdminSettings;
