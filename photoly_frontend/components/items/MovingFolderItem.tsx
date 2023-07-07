import { MinusIcon, AddIcon } from "@chakra-ui/icons";
import {
  Accordion,
  AccordionItem,
  AccordionButton,
  Box,
  AccordionIcon,
  AccordionPanel,
  useToast,
  Button,
  Stack,
  Text,
  VStack,
  HStack,
} from "@chakra-ui/react";
import React, { Dispatch, SetStateAction, useState, useEffect } from "react";
import { AiOutlineFolderOpen } from "react-icons/ai";
import useApi from "../../hooks/useApi";
import useToken from "../../hooks/useToken";

interface movingFolderProps {
  originalFolderId: number;
  currentFolderId: number;
  currentFolderName: string;
  movingFolderId: number;
  setMovingFolderId: Dispatch<SetStateAction<number>>;
  setMovingFolderName: Dispatch<SetStateAction<string>>;
}

interface Namespace {
  nsId: number;
  nsName: string;
  nsParentId: number;
}

const MovingFolderItem = (props: movingFolderProps) => {
  const token = useToken();
  const { post, get } = useApi();
  const toast = useToast();
  const [children, setChildren] = useState<Namespace[]>([]);

  const [selected, setSelected] = useState<Boolean>(false);

  const getChildren = async () => {
    if (props.currentFolderId === props.originalFolderId) setChildren([]);
    const response = await get(
      "/namespace/getChildren?parentId=" + props.currentFolderId,
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data && response.data.msgCode === 200) {
      setChildren(response.data.t);
    } else {
      toast({
        title: `Fail to Get Root`,
        status: "error",
        isClosable: true,
        position: "top",
      });
    }
  };

  useEffect(() => {
    var selected = props.movingFolderId === props.currentFolderId;
    setSelected(selected);
  }, [props.movingFolderId]);

  return (
    <div>
      <Accordion allowMultiple>
        <AccordionItem
          isDisabled={props.currentFolderId === props.originalFolderId}
        >
          <h2>
            <AccordionButton
              onClick={() => {
                getChildren();
                props.setMovingFolderId(props.currentFolderId);
                props.setMovingFolderName(props.currentFolderName);
              }}
              backgroundColor={selected ? "gray.200" : ""}
            >
              <Box flex="1" textAlign="left">
                <HStack>
                  <AiOutlineFolderOpen></AiOutlineFolderOpen>
                  <Text fontSize={"sm"}>{props.currentFolderName}</Text>
                </HStack>
              </Box>
              <AccordionIcon />
            </AccordionButton>
          </h2>
          <AccordionPanel pb={4}>
            <Stack>
              {children.map((namespace) => {
                return (
                  <MovingFolderItem
                    key={namespace.nsId}
                    originalFolderId={props.originalFolderId}
                    movingFolderId={props.movingFolderId}
                    currentFolderId={namespace.nsId}
                    currentFolderName={namespace.nsName}
                    setMovingFolderId={props.setMovingFolderId}
                    setMovingFolderName={props.setMovingFolderName}
                  ></MovingFolderItem>
                );
              })}
            </Stack>
          </AccordionPanel>
        </AccordionItem>
      </Accordion>
    </div>
  );
};

export default MovingFolderItem;
