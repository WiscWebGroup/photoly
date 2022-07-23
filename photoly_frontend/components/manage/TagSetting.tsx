import {
  Center,
  VStack,
  Heading,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Button,
  Popover,
  PopoverArrow,
  PopoverBody,
  PopoverCloseButton,
  PopoverContent,
  PopoverHeader,
  PopoverTrigger,
  AlertDialog,
  AlertDialogBody,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  useDisclosure,
  useToast,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Text,
***REMOVED*** from "@chakra-ui/react";
import React, { useEffect, useState, useRef ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import { AiOutlineDelete ***REMOVED*** from "react-icons/ai";
import { GrAdd, GrUpdate ***REMOVED*** from "react-icons/gr";

interface tag {
  tagId: number;
  userId: number;
  tagName: string;
***REMOVED***

const TagSetting: React.FC = () => {
  const token = useToken();
  const { get, post ***REMOVED*** = useApi();
  const [tagList, setTagList] = useState<tag[]>();

  const [editTagId, setEditTagId] = useState<number>(0);
  const [editTagName, setEditTagName] = useState<string>("");
  const [delTagId, setDelTagId] = useState<number>();

  const [addTagName, setAddTagName] = useState<string>("");

  const {
    isOpen: isOpenEditModal,
    onOpen: onOpenEditModal,
    onClose: onCloseEditModal,
  ***REMOVED*** = useDisclosure();

  const toast = useToast();
  const {
    isOpen: isOpenDeleteConfirm,
    onOpen: onOpenDeleteConfirm,
    onClose: onCloseDeleteConfirm,
  ***REMOVED*** = useDisclosure();
  const cancelRef = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn = useRef<HTMLButtonElement>(null);

  const {
    isOpen: isOpenAddTag,
    onOpen: onOpenAddTag,
    onClose: onCloseAddTag,
  ***REMOVED*** = useDisclosure();

  const getTag = async () => {
    const response = await get("/tag/getAll", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setTagList(response.data.t);
***REMOVED***
  ***REMOVED***;

  const editTag = async (tagId: number) => {
    const response = await post(
      "/tag/update",
  ***REMOVED***
        tagId: tagId,
        tagName: editTagName,
  ***REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
  ***REMOVED***);
      getTag();
***REMOVED***
  ***REMOVED***;

  const delTag = async () => {
    const response = await post(
      "/tag/delete",
  ***REMOVED******REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
        params: {
          tagId: delTagId,
***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Delete Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
  ***REMOVED***);
      getTag();
***REMOVED***
  ***REMOVED***;
  const addTag = async () => {
    const response = await post(
      "/tag/insert",
  ***REMOVED***
        tagName: addTagName,
  ***REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Create Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
  ***REMOVED***);
      getTag();
      setAddTagName("");
      onCloseAddTag();
***REMOVED***
  ***REMOVED***;
  useEffect(() => {
    if (!!token) {
      getTag();
***REMOVED***
  ***REMOVED***, [token]);

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
          >
            Manage My Tag
            <Popover
              isOpen={isOpenAddTag***REMOVED***
              onClose={() => {
                onCloseAddTag();
 ***REMOVED*****REMOVED******REMOVED***
            >
              <PopoverTrigger>
                <Button
                  colorScheme="teal"
                  variant="ghost"
                  ml={4***REMOVED***
                  rightIcon={<GrAdd />***REMOVED***
                  onClick={() => {
                    onOpenAddTag();
***REMOVED*****REMOVED*****REMOVED******REMOVED***
                >
                  New Tag
                </Button>
              </PopoverTrigger>
              <PopoverContent>
                <PopoverArrow />
                <PopoverCloseButton />
                <PopoverHeader>Add Tag</PopoverHeader>
                <PopoverBody>
                  <Input
                    variant="flushed"
                    placeholder="Tag Name"
                    value={addTagName***REMOVED***
                    onChange={(event) => setAddTagName(event.target.value)***REMOVED***
                  />
                  <Button
                    colorScheme="twitter"
                    variant="outline"
                    width={"100%"***REMOVED***
                    mt={5***REMOVED***
                    onClick={() => {
                      addTag();
  ***REMOVED*****REMOVED*****REMOVED******REMOVED***
                  >
                    Create
                  </Button>
                </PopoverBody>
              </PopoverContent>
            </Popover>
          </Heading>
 ***REMOVED*****REMOVED***tagList?.length === 0 ? (
            <Text fontSize="3xl">Nothing Here</Text>
          ) : (
            ""
          )***REMOVED***
          <TableContainer w={"100vw"***REMOVED***>
            <Table variant="simple" size="sm">
              <Thead>
                <Tr>
                  <Th>ID</Th>
                  <Th>Name</Th>
                  <Th>Action</Th>
                </Tr>
              </Thead>
              <Tbody>
  ***REMOVED*****REMOVED*****REMOVED***tagList?.map((tag) => {
                  return (
                    <Tr key={tag.tagId***REMOVED***>
                      <Td>{tag.tagId***REMOVED***</Td>
                      <Td>{tag.tagName***REMOVED***</Td>
                      <Td>
                        <Button
                          leftIcon={<GrUpdate />***REMOVED***
                          onClick={() => {
                            setEditTagName(tag.tagName);
                            setEditTagId(tag.tagId);
                            onOpenEditModal();
   ***REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                        ></Button>

                        <Button
                          leftIcon={<AiOutlineDelete />***REMOVED***
                          onClick={() => {
                            setDelTagId(tag.tagId);
                            onOpenDeleteConfirm();
   ***REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                          disabled={isOpenDeleteConfirm***REMOVED***
                        ></Button>
                      </Td>
                    </Tr>
                  );
   ***REMOVED*****REMOVED***)***REMOVED***
              </Tbody>
            </Table>
          </TableContainer>
        </VStack>

        <AlertDialog
          isOpen={isOpenDeleteConfirm***REMOVED***
          leastDestructiveRef={cancelRef***REMOVED***
          onClose={onCloseDeleteConfirm***REMOVED***
        >
          <AlertDialogOverlay>
            <AlertDialogContent>
              <AlertDialogHeader fontSize="lg" fontWeight="bold">
                Delete API
              </AlertDialogHeader>

              <AlertDialogBody>
                Are you sure? You can&apos;t undo this action afterwards.
              </AlertDialogBody>

              <AlertDialogFooter>
                <Button ref={cancelRefBtn***REMOVED*** onClick={onCloseDeleteConfirm***REMOVED***>
                  Cancel
                </Button>
                <Button
                  colorScheme="red"
                  onClick={() => {
                    delTag();
                    onCloseDeleteConfirm();
***REMOVED*****REMOVED*****REMOVED******REMOVED***
                  ml={3***REMOVED***
                >
                  Delete
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialogOverlay>
        </AlertDialog>
        <Modal isOpen={isOpenEditModal***REMOVED*** onClose={onCloseEditModal***REMOVED***>
          <ModalOverlay />
          <ModalContent>
            <ModalHeader>Edit Tag</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <Input
                variant="flushed"
                placeholder="Tag New Name"
                value={editTagName***REMOVED***
                onChange={(event) => setEditTagName(event.target.value)***REMOVED***
              />
              <br></br>
            </ModalBody>

            <ModalFooter>
              <Button
                colorScheme="blue"
                variant="ghost"
                mr={3***REMOVED***
                onClick={onCloseEditModal***REMOVED***
              >
                Close
              </Button>
              <Button
                colorScheme="twitter"
                variant="outline"
                onClick={() => {
                  editTag(editTagId);
                  onCloseEditModal();
   ***REMOVED*****REMOVED******REMOVED***
              >
                Confirm
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>
      </Center>
    </>
  );
***REMOVED***;

export default TagSetting;
