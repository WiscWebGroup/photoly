import { VStack, Button } from "@chakra-ui/react";
import React from "react";
import { AiOutlineHome } from "react-icons/ai";
import { FiSettings } from "react-icons/fi";
import { RiFileSettingsLine } from "react-icons/ri";

const NavManager = ({
  changeSelection,
}: {
  changeSelection: (num: number) => void;
}) => {
  return (
    <>
      <VStack spacing={0} bg={"white"} h={"calc(100%-4rem)"} w={"15vw"}>
        <Button
          leftIcon={<AiOutlineHome />}
          colorScheme="teal"
          variant="ghost"
          padding={6}
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(0);
          }}
        >
          My Profile
        </Button>
        <Button
          leftIcon={<FiSettings />}
          colorScheme="teal"
          variant="ghost"
          padding={6}
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(1);
          }}
        >
          Settings
        </Button>
        <Button
          leftIcon={<RiFileSettingsLine />}
          colorScheme="teal"
          variant="ghost"
          padding={6}
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(2);
          }}
        >
          Admin Settings
        </Button>
      </VStack>
    </>
  );
};
export default NavManager;
