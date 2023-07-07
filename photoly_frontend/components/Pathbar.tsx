import { Button, Divider, HStack, Icon, Text, VStack } from "@chakra-ui/react";
import { BiChevronRight } from "react-icons/bi";
import React from "react";
import { useSearchData } from "./contexts/SearchContext";
import { AiFillHome } from "react-icons/ai";
import { useAppDispatch } from "../redux/hooks";
import { clearAllFilter } from "../redux/states/searchFilterSlice";

interface PathbarProps {
  children?: React.ReactNode;
}

const Pathbar: React.FC<PathbarProps> = ({ children }) => {
  const searchData = useSearchData();
  const dispatch = useAppDispatch();

  return (
    <VStack pt={8} pl={8} w={"100%"} pr={8}>
      <HStack justifyContent={"space-between"} alignItems={"center"} w={"100%"}>
        <HStack alignSelf={"flex-center"}>
          <Icon as={BiChevronRight} boxSize={8} color={"gray.500"} />
          <Text fontWeight={"bold"} color={"gray.500"}>{searchData.path.map((f) => {
            if (f.name === "/") {
              return "/root";
            }
            return f.name;
          }).join("/")}</Text>
        </HStack>
        <HStack>
          <Button leftIcon={<AiFillHome />} colorScheme="teal" variant="solid"
                  onClick={() => dispatch(clearAllFilter())}>
            Home
          </Button>
        </HStack>
      </HStack>
      <Divider />
      {children}
    </VStack>
  );
};
export default Pathbar;