import {Button, VStack***REMOVED*** from "@chakra-ui/react";
import React from "react";
import {AiOutlineHome, AiOutlineTag***REMOVED*** from "react-icons/ai";
import {RiFileSettingsLine, RiGalleryLine***REMOVED*** from "react-icons/ri";

const NavManager = ({
  changeSelection,
***REMOVED***: {
  changeSelection: (num: number) => void;
***REMOVED***) => {
  return (
    <>
      <VStack spacing={0***REMOVED*** bg={"white"***REMOVED*** h={"calc(100%-4rem)"***REMOVED*** w={"15vw"***REMOVED***>
        <Button
          leftIcon={<AiOutlineHome />***REMOVED***
          colorScheme="teal"
          variant="ghost"
          padding={6***REMOVED***
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(0);
  ***REMOVED******REMOVED***
        >
          My Profile
        </Button>
        <Button
          leftIcon={<AiOutlineTag />***REMOVED***
          colorScheme="teal"
          variant="ghost"
          padding={6***REMOVED***
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(1);
  ***REMOVED******REMOVED***
        >
          Tag Settings
        </Button>
        <Button
          leftIcon={<RiGalleryLine />***REMOVED***
          colorScheme="teal"
          variant="ghost"
          padding={6***REMOVED***
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(2);
  ***REMOVED******REMOVED***
        >
          Gallery Settings
        </Button>
        <Button
          leftIcon={<RiFileSettingsLine />***REMOVED***
          colorScheme="teal"
          variant="ghost"
          padding={6***REMOVED***
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(3);
  ***REMOVED******REMOVED***
        >
          Admin Settings
        </Button>
      </VStack>
    </>
  );
***REMOVED***;
export default NavManager;
