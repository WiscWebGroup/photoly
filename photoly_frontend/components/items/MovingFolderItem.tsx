import { MinusIcon, AddIcon ***REMOVED*** from "@chakra-ui/icons";
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
***REMOVED*** from "@chakra-ui/react";
import React, { Dispatch, SetStateAction, useState, useEffect ***REMOVED*** from "react";
import { AiOutlineFolderOpen ***REMOVED*** from "react-icons/ai";
import useApi from "../../hooks/useApi";
import useToken from "../../hooks/useToken";

interface movingFolderProps {
  originalFolderId: number;
  currentFolderId: number;
  currentFolderName: string;
  movingFolderId: number;
  setMovingFolderId: Dispatch<SetStateAction<number>>;
  setMovingFolderName: Dispatch<SetStateAction<string>>;
***REMOVED***

interface Namespace {
  nsId: number;
  nsName: string;
  nsParentId: number;
***REMOVED***

const MovingFolderItem = (props: movingFolderProps) => {
  const token = useToken();
  const { post, get ***REMOVED*** = useApi();
  const toast = useToast();
  const [children, setChildren] = useState<Namespace[]>([]);

  const [selected, setSelected] = useState<Boolean>(false);

  const getChildren = async () => {
    if (props.currentFolderId === props.originalFolderId) setChildren([]);
    const response = await get(
      "/namespace/getChildren?parentId=" + props.currentFolderId,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data && response.data.msgCode === 200) {
      setChildren(response.data.t);
***REMOVED*** else {
      toast({
        title: `Fail to Get Root`,
        status: "error",
        isClosable: true,
        position: "top",
  ***REMOVED***);
***REMOVED***
  ***REMOVED***;

  useEffect(() => {
    var selected = props.movingFolderId === props.currentFolderId;
    setSelected(selected);
  ***REMOVED***, [props.movingFolderId]);

  return (
    <div>
      <Accordion allowMultiple>
        <AccordionItem
          isDisabled={props.currentFolderId === props.originalFolderId***REMOVED***
        >
          <h2>
            <AccordionButton
              onClick={() => {
                getChildren();
                props.setMovingFolderId(props.currentFolderId);
                props.setMovingFolderName(props.currentFolderName);
 ***REMOVED*****REMOVED******REMOVED***
              backgroundColor={selected ? "gray.200" : ""***REMOVED***
            >
              <Box flex="1" textAlign="left">
                <HStack>
                  <AiOutlineFolderOpen></AiOutlineFolderOpen>
                  <Text fontSize={"sm"***REMOVED***>{props.currentFolderName***REMOVED***</Text>
                </HStack>
              </Box>
              <AccordionIcon />
            </AccordionButton>
          </h2>
          <AccordionPanel pb={4***REMOVED***>
            <Stack>
          ***REMOVED***children.map((namespace) => {
                return (
                  <MovingFolderItem
                    key={namespace.nsId***REMOVED***
                    originalFolderId={props.originalFolderId***REMOVED***
                    movingFolderId={props.movingFolderId***REMOVED***
                    currentFolderId={namespace.nsId***REMOVED***
                    currentFolderName={namespace.nsName***REMOVED***
                    setMovingFolderId={props.setMovingFolderId***REMOVED***
                    setMovingFolderName={props.setMovingFolderName***REMOVED***
                  ></MovingFolderItem>
                );
 ***REMOVED*****REMOVED***)***REMOVED***
            </Stack>
          </AccordionPanel>
        </AccordionItem>
      </Accordion>
    </div>
  );
***REMOVED***;

export default MovingFolderItem;
