import {Center, VStack} from "@chakra-ui/react";
import React from "react";

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
