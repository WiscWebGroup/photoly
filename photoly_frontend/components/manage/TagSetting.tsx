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
} from "@chakra-ui/react";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import { AiOutlineDelete } from "react-icons/ai";
import { GrAdd, GrUpdate } from "react-icons/gr";

interface tag {
  tagId: number;
  userId: number;
  tagName: string;
}

const TagSetting: React.FC = () => {
  const token = useToken();
  const { get, post } = useApi();
  const [tagList, setTagList] = useState<tag[]>();

  const [editTagId, setEditTagId] = useState<number>(0);
  const [editTagName, setEditTagName] = useState<string>("");
  const [delTagId, setDelTagId] = useState<number>();

  const [addTagName, setAddTagName] = useState<string>("");

  const {
    isOpen: isOpenEditModal,
    onOpen: onOpenEditModal,
    onClose: onCloseEditModal,
  } = useDisclosure();

  const toast = useToast();
  const {
    isOpen: isOpenDeleteConfirm,
    onOpen: onOpenDeleteConfirm,
    onClose: onCloseDeleteConfirm,
  } = useDisclosure();
  const cancelRef = useRef<HTMLDivElement | HTMLButtonElement>(null);
  const cancelRefBtn = useRef<HTMLButtonElement>(null);

  const {
    isOpen: isOpenAddTag,
    onOpen: onOpenAddTag,
    onClose: onCloseAddTag,
  } = useDisclosure();

  const getTag = async () => {
    const response = await get("/tag/getAll", {
      headers: { "HRD-token": token },
    });
    if (!!response && response.data.msgCode === 200) {
      setTagList(response.data.t);
    }
  };

  const editTag = async (tagId: number) => {
    const response = await post(
      "/tag/update",
      {
        tagId: tagId,
        tagName: editTagName,
      },
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Update Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
      });
      getTag();
    }
  };

  const delTag = async () => {
    const response = await post(
      "/tag/delete",
      {},
      {
        headers: { "HRD-token": token },
        params: {
          tagId: delTagId,
        },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Delete Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
      });
      getTag();
    }
  };
  const addTag = async () => {
    const response = await post(
      "/tag/insert",
      {
        tagName: addTagName,
      },
      {
        headers: { "HRD-token": token },
      }
    );
    if (!!response && response.data.msgCode === 200) {
      toast({
        title: `Create Successful`,
        status: "success",
        isClosable: true,
        duration: 3000,
        position: "top",
      });
      getTag();
      setAddTagName("");
      onCloseAddTag();
    }
  };
  useEffect(() => {
    if (!!token) {
      getTag();
    }
  }, [token]);

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
          {tagList?.length === 0 ? (
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
                {tagList?.map((tag) => {
                  return (
                    <Tr key={tag.tagId}>
                      <Td>{tag.tagId}</Td>
                      <Td>{tag.tagName}</Td>
                      <Td>
                        <Button
                          leftIcon={<GrUpdate />}
                          onClick={() => {
                            setEditTagName(tag.tagName);
                            setEditTagId(tag.tagId);
                            onOpenEditModal();
                          }}
                        ></Button>

                        <Button
                          leftIcon={<AiOutlineDelete />}
                          onClick={() => {
                            setDelTagId(tag.tagId);
                            onOpenDeleteConfirm();
                          }}
                          disabled={isOpenDeleteConfirm}
                        ></Button>
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
                Delete API
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
                onClick={() => {
                  editTag(editTagId);
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
