import {
  Center,
  VStack,
  Divider,
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
  Select,
  Box,
} from "@chakra-ui/react";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import { AiOutlineDelete, AiOutlineFlag, AiOutlineStar } from "react-icons/ai";
import { TbAlbum } from "react-icons/tb";
import { FaUmbrellaBeach } from "react-icons/fa";
import { HiOutlineAcademicCap } from "react-icons/hi";
import { GiMountainCave } from "react-icons/gi";
import { GrAdd, GrFavorite, GrUpdate } from "react-icons/gr";
import { IconType } from "react-icons/lib";
import { BiPaperPlane } from "react-icons/bi";

interface gallery {
  gaId: number;
  gaName: string;
  userId: number;
  createDate: string;
  coverId: number;
  coverColor: string;
}

const GallerySetting: React.FC = () => {
  const token = useToken();
  const { get, post } = useApi();
  const [galleryList, setGalleryList] = useState<gallery[]>();

  const [editGalleryId, setEditGalleryId] = useState<number>(0);
  const [editGalleryName, setEditGalleryName] = useState<string>("");
  const [editGalleryColor, setEditGalleryColor] = useState<string>("#FFF");
  const [editGalleryCover, setEditGalleryCover] = useState<number>(0);

  const [delGalleryId, setDelGalleryId] = useState<number>();

  const [addGalleryName, setAddGalleryName] = useState<string>("");

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
    isOpen: isOpenAddGallery,
    onOpen: onOpenAddGallery,
    onClose: onCloseAddGallery,
  } = useDisclosure();

  const getGallery = async () => {
    const response = await get("/gallery/getAll", {
      headers: { "HRD-token": token },
    });
    if (!!response && response.data.msgCode === 200) {
      setGalleryList(response.data.t);
    }
  };

  const editGallery = async (galleryId: number) => {
    const response = await post(
      "/gallery/update",
      {
        gaId: galleryId,
        gaName: editGalleryName,
        coverColor: editGalleryColor,
        coverId: editGalleryCover,
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
      getGallery();
    }
  };

  const delGallery = async () => {
    const response = await post(
      "/gallery/delete",
      {},
      {
        headers: { "HRD-token": token },
        params: {
          gaId: delGalleryId,
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
      getGallery();
    }
  };
  const addGallery = async () => {
    const response = await post(
      "/gallery/insert",
      {
        gaName: addGalleryName,
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
      getGallery();
      setAddGalleryName("");
      onCloseAddGallery();
    }
  };

  const getCoverIcon = (coverId: number | null) => {
    if (coverId === null || coverId === 0) {
      return <TbAlbum />;
    } else if (coverId === 1) {
      return <AiOutlineStar />;
    } else if (coverId === 2) {
      return <GrFavorite />;
    } else if (coverId === 3) {
      return <FaUmbrellaBeach />;
    } else if (coverId === 4) {
      return <BiPaperPlane />;
    } else if (coverId === 5) {
      return <AiOutlineFlag />;
    } else if (coverId === 6) {
      return <HiOutlineAcademicCap />;
    } else if (coverId === 7) {
      return <GiMountainCave />;
    }
  };

  useEffect(() => {
    if (!!token) {
      getGallery();
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
          <Divider />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"}
            alignSelf={"flex-start"}
          >
            Manage My Gallery
            <Popover
              isOpen={isOpenAddGallery}
              onClose={() => {
                onCloseAddGallery();
              }}
            >
              <PopoverTrigger>
                <Button
                  colorScheme="teal"
                  variant="ghost"
                  ml={4}
                  rightIcon={<GrAdd />}
                  onClick={() => {
                    onOpenAddGallery();
                  }}
                >
                  New Gallery
                </Button>
              </PopoverTrigger>
              <PopoverContent>
                <PopoverArrow />
                <PopoverCloseButton />
                <PopoverHeader>Add Gallery</PopoverHeader>
                <PopoverBody>
                  <Input
                    variant="flushed"
                    placeholder="Gallery Name"
                    value={addGalleryName}
                    onChange={(event) => setAddGalleryName(event.target.value)}
                  />
                  <Button
                    colorScheme="twitter"
                    variant="outline"
                    width={"100%"}
                    mt={5}
                    onClick={() => {
                      addGallery();
                    }}
                  >
                    Create
                  </Button>
                </PopoverBody>
              </PopoverContent>
            </Popover>
          </Heading>
          {galleryList?.length === 0 ? (
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
                  <Th>Color</Th>
                  <Th>Cover</Th>
                  <Th>Time Crd</Th>
                  <Th>Action</Th>
                </Tr>
              </Thead>
              <Tbody>
                {galleryList?.map((gallery) => {
                  return (
                    <Tr key={gallery.gaId}>
                      <Td>{gallery.gaId}</Td>
                      <Td>{gallery.gaName}</Td>
                      <Td>
                        <Box
                          w="40px"
                          h="40px"
                          bg={gallery.coverColor}
                          borderRadius="25"
                        ></Box>
                      </Td>
                      <Td>{getCoverIcon(gallery.coverId)}</Td>
                      <Td>{gallery.createDate.substr(0, 10)}</Td>
                      <Td>
                        <Button
                          leftIcon={<GrUpdate />}
                          onClick={() => {
                            setEditGalleryName(gallery.gaName);
                            setEditGalleryId(gallery.gaId);
                            setEditGalleryColor(gallery.coverColor);
                            if (gallery.coverId === null) {
                              setEditGalleryCover(-1);
                            } else {
                              setEditGalleryCover(gallery.coverId);
                            }

                            onOpenEditModal();
                          }}
                        ></Button>

                        <Button
                          leftIcon={<AiOutlineDelete />}
                          onClick={() => {
                            setDelGalleryId(gallery.gaId);
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
                Are you sure? You can't undo this action afterwards.
              </AlertDialogBody>

              <AlertDialogFooter>
                <Button ref={cancelRefBtn} onClick={onCloseDeleteConfirm}>
                  Cancel
                </Button>
                <Button
                  colorScheme="red"
                  onClick={() => {
                    delGallery();
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
            <ModalHeader>Edit Gallery</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <Input
                variant="flushed"
                placeholder="Gallery New Name"
                value={editGalleryName}
                onChange={(event) => setEditGalleryName(event.target.value)}
              />
              <Input
                variant="filled"
                type={"color"}
                value={editGalleryColor}
                onChange={(event) => setEditGalleryColor(event.target.value)}
                mt={3}
              />
              <Select
                placeholder="Icon Format"
                mt={3}
                value={editGalleryCover}
                onChange={(e) => {
                  setEditGalleryCover(parseInt(e.target.value));
                }}
              >
                <option value={0}>Default</option>
                <option value={1}>Star</option>
                <option value={2}>Favorite</option>
                <option value={3}>Beach</option>
                <option value={4}>Plane</option>
                <option value={5}>Flag</option>
                <option value={6}>Academic</option>
                <option value={7}>Mountain</option>
              </Select>
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
                  editGallery(editGalleryId);
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

export default GallerySetting;
