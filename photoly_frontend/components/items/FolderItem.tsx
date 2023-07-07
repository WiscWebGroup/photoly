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
} from "@chakra-ui/react";
import { ChangeEvent, useState } from "react";
import { AiOutlineFolderOpen } from "react-icons/ai";
import useApi from "../../hooks/useApi";
import useToken from "../../hooks/useToken";
import { ContextMenu } from "../ContextMenu";
import FolderContextMenu from "../contextMenus/FolderContextMenu";
import { useOpenFolder } from "../contexts/SearchContext";
import MovingFolderItem from "./MovingFolderItem";
import { useAppDispatch } from "../../redux/hooks";
import { setFolder } from "../../redux/states/searchFilterSlice";

interface FolderItemProps {
  id: number;
  name: string;
  parentId: number;
}

const FolderItem = ({ id, name, parentId }: FolderItemProps) => {
  const openFolder = useOpenFolder();
  const [newName, setNewName] = useState("");
  const [isError, setIsError] = useState(true);
  const [isLoading, setIsLoading] = useState(false);
  const {
    isOpen: isRenameOpen,
    onOpen: onRenameOpen,
    onClose: onRenameClose,
  } = useDisclosure();
  const {
    isOpen: isMoveOpen,
    onOpen: onMoveOpen,
    onClose: onMoveClose,
  } = useDisclosure();
  const [movingToParentFolderId, setMovingToParentFolderId] = useState(-1);
  const [movingToParentFolderName, setMovingToParentFolderName] = useState("");

  const token = useToken();
  const { post, get } = useApi();
  const toast = useToast();

  const dispatch = useAppDispatch()

  const deleteRequest = async () => {
    const response = await post("/namespace/delete", null, {
      headers: { "HRD-token": token },
      params: {
        nsId: id,
      },
    });

    if (!!response && response.data && response.data.msgCode === 200) {
      openFolder(parentId);
    } else {
      toast({
        title: `Failed to delete the folder "${name}"`,
        status: "error",
        isClosable: true,
        position: "top",
      });
    }
  };

  const handleFolderRename = (e: ChangeEvent<HTMLInputElement>) => {
    const value = e.currentTarget.value;
    setIsError(value.length === 0 || value === "/");
    setNewName(value);
  };

  const handleResetNewName = () => {
    setNewName("");
    setIsError(true);
    onRenameClose();
  };

  const renameRequest = async () => {
    setIsLoading(true);
    const response = await post(
      "/namespace/updateName",
      {
        nsId: id,
        nsName: newName,
      },
      {
        headers: { "HRD-token": token },
      }
    );

    if (!!response && response.data && response.data.msgCode === 200) {
      openFolder(parentId);
    } else {
      toast({
        title: `Failed to rename the folder "${name}" to "${newName}"`,
        status: "error",
        isClosable: true,
        position: "top",
      });
    }
    setIsLoading(false);
    handleResetNewName();
  };

  // TODO: handle moveto

  const getRoot = async () => {
    const response = await get("/namespace/getRoot", {
      headers: { "HRD-token": token },
    });
    if (!!response && response.data && response.data.msgCode === 200) {
      setRootId(response.data.t.nsId);
    } else {
      toast({
        title: `Fail to Get Root2`,
        status: "error",
        isClosable: true,
        position: "top",
      });
    }
  };

  const [rootId, setRootId] = useState(-1);

  const handleMoveFolder = () => {
    setMovingToParentFolderId(-1);
    setMovingToParentFolderName("/");
    onMoveClose();
  };

  const moveToOpen = () => {
    getRoot();
    setMovingToParentFolderName("/");
    setMovingToParentFolderId(rootId);
    onMoveOpen();
  };

  const moveFolder = async () => {
    setIsLoading(true);
    const response = await post(
      "/namespace/updateParent",
      {
        nsId: id,
        nsParentId: movingToParentFolderId,
      },
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data && response.data.msgCode === 200) {
      toast({
        title: `Success`,
        status: "success",
        isClosable: true,
        position: "top",
        duration: 1000,
      });
      openFolder(parentId);
    } else {
      toast({
        title: `Fail to move the folder`,
        status: "error",
        isClosable: true,
        position: "top",
      });
    }
    handleMoveFolder();
    setIsLoading(false);
  };

  return (
    <ContextMenu<HTMLDivElement>
      stopPropagation
      renderMenu={() => (
        <FolderContextMenu
          handleDelete={deleteRequest}
          handleRename={onRenameOpen}
          handleMoveTo={moveToOpen}
        />
      )}
    >
      {(ref) => (
        <Box ref={ref} w={64} overflow="hidden" position="relative" m={2}>
          <Button
            leftIcon={<AiOutlineFolderOpen />}
            colorScheme="gray"
            variant="solid"
            w={"100%"}
            onClick={() => dispatch(setFolder(id))}
          >
            {name}
          </Button>

          <Modal isOpen={isRenameOpen} onClose={handleResetNewName}>
            <ModalOverlay />
            <ModalContent>
              <ModalHeader>Rename a folder</ModalHeader>
              <ModalCloseButton />
              <ModalBody>
                <FormControl>
                  <FormLabel>Old folder name</FormLabel>
                  <Input type="text" defaultValue={name} readOnly />
                </FormControl>
                <FormControl isInvalid={isError}>
                  <FormLabel>New folder name</FormLabel>
                  <Input
                    type="text"
                    placeholder="New folder name"
                    onChange={handleFolderRename}
                  />
                  {isError && (
                    <FormErrorMessage>
                      Folder name must not be empty or &quot;/&quot;
                    </FormErrorMessage>
                  )}
                </FormControl>
              </ModalBody>
              <ModalFooter>
                <Button
                  colorScheme="teal"
                  mr={3}
                  onClick={renameRequest}
                  disabled={isError}
                  isLoading={isLoading}
                >
                  Update
                </Button>
                <Button variant="ghost" mr={3} onClick={handleResetNewName}>
                  Cancel
                </Button>
              </ModalFooter>
            </ModalContent>
          </Modal>

          <Modal isOpen={isMoveOpen} onClose={handleMoveFolder}>
            <ModalOverlay />
            <ModalContent>
              <ModalHeader>Move Folder</ModalHeader>
              <ModalCloseButton />
              <ModalBody>
                <Text fontSize="md">
                  Current Selected: {movingToParentFolderName}
                </Text>
                <MovingFolderItem
                  originalFolderId={id}
                  movingFolderId={movingToParentFolderId}
                  setMovingFolderId={setMovingToParentFolderId}
                  setMovingFolderName={setMovingToParentFolderName}
                  currentFolderId={rootId}
                  currentFolderName="/"
                ></MovingFolderItem>
              </ModalBody>
              <ModalFooter>
                <Button
                  colorScheme="teal"
                  mr={3}
                  onClick={moveFolder}
                  isLoading={isLoading}
                >
                  Update
                </Button>
                <Button variant="ghost" mr={3} onClick={handleMoveFolder}>
                  Cancel
                </Button>
              </ModalFooter>
            </ModalContent>
          </Modal>
        </Box>
      )}
    </ContextMenu>
  );
};

export default FolderItem;
