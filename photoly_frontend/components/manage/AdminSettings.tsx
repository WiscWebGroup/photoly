import { Center, VStack } from "@chakra-ui/react";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";

const AdminSettings: React.FC = () => {
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
          <p>Admin Settings</p>
        </VStack>
      </Center>
    </>
  );
};

export default AdminSettings;
