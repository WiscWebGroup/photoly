import { Button, Divider, HStack, Icon, Text, VStack ***REMOVED*** from "@chakra-ui/react";
import { BiChevronRight ***REMOVED*** from "react-icons/bi";
import React from "react";
import { useSearchData ***REMOVED*** from "./contexts/SearchContext";
import { AiFillHome ***REMOVED*** from "react-icons/ai";
import { useAppDispatch ***REMOVED*** from "../redux/hooks";
import { clearAllFilter ***REMOVED*** from "../redux/states/searchFilterSlice";

interface PathbarProps {
  children?: React.ReactNode;
***REMOVED***

const Pathbar: React.FC<PathbarProps> = ({ children ***REMOVED***) => {
  const searchData = useSearchData();
  const dispatch = useAppDispatch();

  return (
    <VStack pt={8***REMOVED*** pl={8***REMOVED*** w={"100%"***REMOVED*** pr={8***REMOVED***>
      <HStack justifyContent={"space-between"***REMOVED*** alignItems={"center"***REMOVED*** w={"100%"***REMOVED***>
        <HStack alignSelf={"flex-center"***REMOVED***>
          <Icon as={BiChevronRight***REMOVED*** boxSize={8***REMOVED*** color={"gray.500"***REMOVED*** />
          <Text fontWeight={"bold"***REMOVED*** color={"gray.500"***REMOVED***>{searchData.path.map((f) => {
            if (f.name === "/") {
              return "/root";
    ***REMOVED***
            return f.name;
  ***REMOVED***).join("/")***REMOVED***</Text>
        </HStack>
        <HStack>
          <Button leftIcon={<AiFillHome />***REMOVED*** colorScheme="teal" variant="solid"
                  onClick={() => dispatch(clearAllFilter())***REMOVED***>
            Home
          </Button>
        </HStack>
      </HStack>
      <Divider />
  ***REMOVED***children***REMOVED***
    </VStack>
  );
***REMOVED***;
export default Pathbar;