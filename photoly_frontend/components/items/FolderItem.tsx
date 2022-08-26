import {
  Box,
  Button,
  FormControl,
  FormErrorMessage,
  FormLabel,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Text,
  useDisclosure,
  useToast,
***REMOVED*** from "@chakra-ui/react";
import { ChangeEvent, useState ***REMOVED*** from "react";
import { AiOutlineFolderOpen ***REMOVED*** from "react-icons/ai";
import useApi from "../../hooks/useApi";
import useToken from "../../hooks/useToken";
import { ContextMenu ***REMOVED*** from "../ContextMenu";
import FolderContextMenu from "../contextMenus/FolderContextMenu";
import { useOpenFolder ***REMOVED*** from "../contexts/SearchContext";
import MovingFolderItem from "./MovingFolderItem";

interface FolderItemProps {
  id: number;
  name: string;
  parentId: number;
***REMOVED***

const FolderItem = ({ id, name, parentId ***REMOVED***: FolderItemProps) => {
  const openFolder = useOpenFolder();
  const [newName, setNewName] = useState("");
  const [isError, setIsError] = useState(true);
  const [isLoading, setIsLoading] = useState(false);
  const {
    isOpen: isRenameOpen,
    onOpen: onRenameOpen,
    onClose: onRenameClose,
  ***REMOVED*** = useDisclosure();
  const {
    isOpen: isMoveOpen,
    onOpen: onMoveOpen,
    onClose: onMoveClose,
  ***REMOVED*** = useDisclosure();
  const [movingToParentFolderId, setMovingToParentFolderId] = useState(-1);
  const [movingToParentFolderName, setMovingToParentFolderName] = useState("");

  const token = useToken();
  const { post, get ***REMOVED*** = useApi();
  const toast = useToast();

  const deleteRequest = async () => {
    const response = await post("/namespace/delete", null, {
      headers: { "HRD-token": token ***REMOVED***,
      params: {
        nsId: id,
  ***REMOVED***,
***REMOVED***);

    if (!!response && response.data && response.data.msgCode === 200) {
      openFolder(parentId);
***REMOVED*** else {
      toast({
        title: `Failed to delete the folder "${name***REMOVED***"`,
        status: "error",
        isClosable: true,
        position: "top",
  ***REMOVED***);
***REMOVED***
  ***REMOVED***;

  const handleFolderRename = (e: ChangeEvent<HTMLInputElement>) => {
    const value = e.currentTarget.value;
    setIsError(value.length === 0 || value === "/");
    setNewName(value);
  ***REMOVED***;

  const handleResetNewName = () => {
    setNewName("");
    setIsError(true);
    onRenameClose();
  ***REMOVED***;

  const renameRequest = async () => {
    setIsLoading(true);
    const response = await post(
      "/namespace/updateName",
  ***REMOVED***
        nsId: id,
        nsName: newName,
  ***REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );

    if (!!response && response.data && response.data.msgCode === 200) {
      openFolder(parentId);
***REMOVED*** else {
      toast({
        title: `Failed to rename the folder "${name***REMOVED***" to "${newName***REMOVED***"`,
        status: "error",
        isClosable: true,
        position: "top",
  ***REMOVED***);
***REMOVED***
    setIsLoading(false);
    handleResetNewName();
  ***REMOVED***;

  // TODO: handle moveto

  const getRoot = async () => {
    const response = await get("/namespace/getRoot", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data && response.data.msgCode === 200) {
      setRootId(response.data.t.nsId);
***REMOVED*** else {
      toast({
        title: `Fail to Get Root2`,
        status: "error",
        isClosable: true,
        position: "top",
  ***REMOVED***);
***REMOVED***
  ***REMOVED***;

  const [rootId, setRootId] = useState(-1);

  const handleMoveFolder = () => {
    setMovingToParentFolderId(-1);
    setMovingToParentFolderName("/");
    onMoveClose();
  ***REMOVED***;

  const moveToOpen = () => {
    getRoot();
    setMovingToParentFolderName("/");
    setMovingToParentFolderId(rootId);
    onMoveOpen();
  ***REMOVED***;

  const moveFolder = async () => {
    setIsLoading(true);
    const response = await post(
      "/namespace/updateParent",
  ***REMOVED***
        nsId: id,
        nsParentId: movingToParentFolderId,
  ***REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
  ***REMOVED***
    );
    if (!!response && response.data && response.data.msgCode === 200) {
      toast({
        title: `Success`,
        status: "success",
        isClosable: true,
        position: "top",
        duration: 1000,
  ***REMOVED***);
      openFolder(parentId);
***REMOVED*** else {
      toast({
        title: `Fail to move the folder`,
        status: "error",
        isClosable: true,
        position: "top",
  ***REMOVED***);
***REMOVED***
    handleMoveFolder();
    setIsLoading(false);
  ***REMOVED***;

  return (
    <ContextMenu<HTMLDivElement>
      stopPropagation
      renderMenu={() => (
        <FolderContextMenu
          handleDelete={deleteRequest***REMOVED***
          handleRename={onRenameOpen***REMOVED***
          handleMoveTo={moveToOpen***REMOVED***
        />
      )***REMOVED***
    >
  ***REMOVED***(ref) => (
        <Box ref={ref***REMOVED*** w={64***REMOVED*** overflow="hidden" position="relative" m={2***REMOVED***>
          <Button
            leftIcon={<AiOutlineFolderOpen />***REMOVED***
            colorScheme="gray"
            variant="solid"
            w={"100%"***REMOVED***
            onClick={() => openFolder(id)***REMOVED***
          >
   ***REMOVED*****REMOVED***name***REMOVED***
          </Button>

          <Modal isOpen={isRenameOpen***REMOVED*** onClose={handleResetNewName***REMOVED***>
            <ModalOverlay />
            <ModalContent>
              <ModalHeader>Rename a folder</ModalHeader>
              <ModalCloseButton />
              <ModalBody>
                <FormControl>
                  <FormLabel>Old folder name</FormLabel>
                  <Input type="text" defaultValue={name***REMOVED*** readOnly />
                </FormControl>
                <FormControl isInvalid={isError***REMOVED***>
                  <FormLabel>New folder name</FormLabel>
                  <Input
                    type="text"
                    placeholder="New folder name"
                    onChange={handleFolderRename***REMOVED***
                  />
    ***REMOVED*****REMOVED*****REMOVED***isError && (
                    <FormErrorMessage>
                      Folder name must not be empty or &quot;/&quot;
                    </FormErrorMessage>
                  )***REMOVED***
                </FormControl>
              </ModalBody>
              <ModalFooter>
                <Button
                  colorScheme="teal"
                  mr={3***REMOVED***
                  onClick={renameRequest***REMOVED***
                  disabled={isError***REMOVED***
                  isLoading={isLoading***REMOVED***
                >
                  Update
                </Button>
                <Button variant="ghost" mr={3***REMOVED*** onClick={handleResetNewName***REMOVED***>
                  Cancel
                </Button>
              </ModalFooter>
            </ModalContent>
          </Modal>

          <Modal isOpen={isMoveOpen***REMOVED*** onClose={handleMoveFolder***REMOVED***>
            <ModalOverlay />
            <ModalContent>
              <ModalHeader>Move Folder</ModalHeader>
              <ModalCloseButton />
              <ModalBody>
                <Text fontSize="md">
                  Current Selected: {movingToParentFolderName***REMOVED***
                </Text>
                <MovingFolderItem
                  originalFolderId={id***REMOVED***
                  movingFolderId={movingToParentFolderId***REMOVED***
                  setMovingFolderId={setMovingToParentFolderId***REMOVED***
                  setMovingFolderName={setMovingToParentFolderName***REMOVED***
                  currentFolderId={rootId***REMOVED***
                  currentFolderName="/"
                ></MovingFolderItem>
              </ModalBody>
              <ModalFooter>
                <Button
                  colorScheme="teal"
                  mr={3***REMOVED***
                  onClick={moveFolder***REMOVED***
                  isLoading={isLoading***REMOVED***
                >
                  Update
                </Button>
                <Button variant="ghost" mr={3***REMOVED*** onClick={handleMoveFolder***REMOVED***>
                  Cancel
                </Button>
              </ModalFooter>
            </ModalContent>
          </Modal>
        </Box>
      )***REMOVED***
    </ContextMenu>
  );
***REMOVED***;

export default FolderItem;
