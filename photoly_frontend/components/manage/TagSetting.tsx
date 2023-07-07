import {
  AlertDialog,
  AlertDialogBody,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  Button,
  Center,
  Heading, IconButton,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Popover,
  PopoverArrow,
  PopoverBody,
  PopoverCloseButton,
  PopoverContent,
  PopoverHeader,
  PopoverTrigger,
  Table,
  TableContainer,
  Tbody,
  Td,
  Text,
  Th,
  Thead,
  Tr,
  useDisclosure,
  useToast,
  VStack
} from "@chakra-ui/react";
import React, { useRef, useState } from "react";
import { AiOutlineDelete } from "react-icons/ai";
import { GrAdd, GrUpdate } from "react-icons/gr";
import {
  useDeleteTagMutation,
  useGetAllTagsQuery,
  useInsertTagMutation,
  useRenameTagMutation
} from "../../redux/api/tagSlice";

interface tag {
  tagId: number;
  userId: number;
  tagName: string;
}

const TagSetting: React.FC = () => {
  const [editTagId, setEditTagId] = useState<number>(0);
  const [editTagName, setEditTagName] = useState<string>("");
  const [delTagId, setDelTagId] = useState<number>(0);

  const [addTagName, setAddTagName] = useState<string>("");
  const { data: tags, error, isLoading } = useGetAllTagsQuery();
  const [insertTrigger] = useInsertTagMutation();
  const [deleteTrigger] = useDeleteTagMutation();
  const [renameTrigger] = useRenameTagMutation();

  const {
    isOpen: isOpenEditModal,
    onOpen: onOpenEditModal,
    onClose: onCloseEditModal
  } = useDisclosure();

  const toast = useToast();
  const {
    isOpen: isOpenDeleteConfirm,
    onOpen: onOpenDeleteConfirm,
    onClose: onCloseDeleteConfirm
  } = useDisclosure();
  const cancelRef = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn = useRef<HTMLButtonElement>(null);

  const {
    isOpen: isOpenAddTag,
    onOpen: onOpenAddTag,
    onClose: onCloseAddTag
  } = useDisclosure();


  const editTag = async (tagId: number) => {
    renameTrigger({ tag_id: tagId, new_name: editTagName });
    toast({
      title: `Update Successful`,
      status: "success",
      isClosable: true,
      duration: 3000,
      position: "top"
    });
  };

  const delTag = () => {
    deleteTrigger(delTagId);
    toast({
      title: `Delete Successful`,
      status: "success",
      isClosable: true,
      duration: 3000,
      position: "top"
    });
  };
  const addTag = () => {
    insertTrigger(addTagName);
    toast({
      title: `Create Successful`,
      status: "success",
      isClosable: true,
      duration: 3000,
      position: "top"
    });
    setAddTagName("");
    onCloseAddTag();
  };

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
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
          >
            Manage My Tag
            <Popover
              isOpen={isOpenAddTag}
              onClose={() => {
                onCloseAddTag();
              }}
            >
              <PopoverTrigger>
                <Button
                  colorScheme="teal"
                  variant="ghost"
                  ml={4}
                  rightIcon={<GrAdd />}
                  onClick={() => {
                    onOpenAddTag();
                  }}
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
                    value={addTagName}
                    onChange={(event) => setAddTagName(event.target.value)}
                  />
                  <Button
                    colorScheme="twitter"
                    variant="outline"
                    width={"100%"}
                    mt={5}
                    onClick={() => {
                      addTag();
                    }}
                  >
                    Create
                  </Button>
                </PopoverBody>
              </PopoverContent>
            </Popover>
          </Heading>
          {tags?.length === 0 ? (
            <Text fontSize="3xl">Nothing Here</Text>
          ) : (
            ""
          )}
          <TableContainer w={"100vw"}>
            <Table variant="simple" size="sm">
              <Thead>
                <Tr>
                  <Th>ID</Th>
                  <Th>Name</Th>
                  <Th>Action</Th>
                </Tr>
              </Thead>
              <Tbody>
                {tags?.map((tag) => {
                  return (
                    <Tr key={tag.tagId}>
                      <Td>{tag.tagId}</Td>
                      <Td>{tag.tagName}</Td>
                      <Td>
                        <IconButton
                          icon={<GrUpdate />}
                          onClick={() => {
                            setEditTagName(tag.tagName);
                            setEditTagId(tag.tagId);
                            onOpenEditModal();
                          }}
                         aria-label={"rename"}
                         mr={2}>

                        </IconButton>

                        <IconButton
                          icon={<AiOutlineDelete />}
                          onClick={() => {
                            setDelTagId(tag.tagId);
                            onOpenDeleteConfirm();
                          }}
                          disabled={isOpenDeleteConfirm}
                          aria-label={"delete"}
                        ></IconButton>
                      </Td>
                    </Tr>
                  );
                })}
              </Tbody>
            </Table>
          </TableContainer>
        </VStack>

        <AlertDialog
          isOpen={isOpenDeleteConfirm}
          leastDestructiveRef={cancelRef}
          onClose={onCloseDeleteConfirm}
        >
          <AlertDialogOverlay>
            <AlertDialogContent>
              <AlertDialogHeader fontSize="lg" fontWeight="bold">
                Delete Tag
              </AlertDialogHeader>

              <AlertDialogBody>
                Are you sure? You can&apos;t undo this action afterwards.
              </AlertDialogBody>

              <AlertDialogFooter>
                <Button ref={cancelRefBtn} onClick={onCloseDeleteConfirm}>
                  Cancel
                </Button>
                <Button
                  colorScheme="red"
                  onClick={() => {
                    delTag();
                    onCloseDeleteConfirm();
                  }}
                  ml={3}
                >
                  Delete
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialogOverlay>
        </AlertDialog>
        <Modal isOpen={isOpenEditModal} onClose={onCloseEditModal}>
          <ModalOverlay />
          <ModalContent>
            <ModalHeader>Edit Tag</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <Input
                variant="flushed"
                placeholder="Tag New Name"
                value={editTagName}
                onChange={(event) => setEditTagName(event.target.value)}
              />
              <br></br>
            </ModalBody>

            <ModalFooter>
              <Button
                colorScheme="blue"
                variant="ghost"
                mr={3}
                onClick={onCloseEditModal}
              >
                Close
              </Button>
              <Button
                colorScheme="twitter"
                variant="outline"
                onClick={async () => {
                  await editTag(editTagId);
                  onCloseEditModal();
                }}
              >
                Confirm
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>
      </Center>
    </>
  );
};

export default TagSetting;
