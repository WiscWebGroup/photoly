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
***REMOVED*** from "@chakra-ui/react";
import React, { useEffect, useState, useRef ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import { AiOutlineDelete, AiOutlineFlag, AiOutlineStar ***REMOVED*** from "react-icons/ai";
import { TbAlbum ***REMOVED*** from "react-icons/tb";
import { FaUmbrellaBeach ***REMOVED*** from "react-icons/fa";
import { HiOutlineAcademicCap ***REMOVED*** from "react-icons/hi";
import { GiMountainCave ***REMOVED*** from "react-icons/gi";
import { GrAdd, GrFavorite, GrUpdate ***REMOVED*** from "react-icons/gr";
import { IconType ***REMOVED*** from "react-icons/lib";
import { BiPaperPlane ***REMOVED*** from "react-icons/bi";

interface gallery {
  gaId: number;
  gaName: string;
  userId: number;
  createDate: string;
  coverId: number;
  coverColor: string;
***REMOVED***

const GallerySetting: React.FC = () => {
  const token = useToken();
  const { get, post ***REMOVED*** = useApi();
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
    isOpen: isOpenAddGallery,
    onOpen: onOpenAddGallery,
    onClose: onCloseAddGallery,
  ***REMOVED*** = useDisclosure();

  const getGallery = async () => {
    const response = await get("/gallery/getAll", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setGalleryList(response.data.t);
***REMOVED***
  ***REMOVED***;

  const editGallery = async (galleryId: number) => {
    const response = await post(
      "/gallery/update",
  ***REMOVED***
        gaId: galleryId,
        gaName: editGalleryName,
        coverColor: editGalleryColor,
        coverId: editGalleryCover,
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
      getGallery();
***REMOVED***
  ***REMOVED***;

  const delGallery = async () => {
    const response = await post(
      "/gallery/delete",
  ***REMOVED******REMOVED***,
  ***REMOVED***
        headers: { "HRD-token": token ***REMOVED***,
        params: {
          gaId: delGalleryId,
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
      getGallery();
***REMOVED***
  ***REMOVED***;
  const addGallery = async () => {
    const response = await post(
      "/gallery/insert",
  ***REMOVED***
        gaName: addGalleryName,
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
      getGallery();
      setAddGalleryName("");
      onCloseAddGallery();
***REMOVED***
  ***REMOVED***;

  const getCoverIcon = (coverId: number | null) => {
    if (coverId === null || coverId === 0) {
      return <TbAlbum />;
***REMOVED*** else if (coverId === 1) {
      return <AiOutlineStar />;
***REMOVED*** else if (coverId === 2) {
      return <GrFavorite />;
***REMOVED*** else if (coverId === 3) {
      return <FaUmbrellaBeach />;
***REMOVED*** else if (coverId === 4) {
      return <BiPaperPlane />;
***REMOVED*** else if (coverId === 5) {
      return <AiOutlineFlag />;
***REMOVED*** else if (coverId === 6) {
      return <HiOutlineAcademicCap />;
***REMOVED*** else if (coverId === 7) {
      return <GiMountainCave />;
***REMOVED***
  ***REMOVED***;

  useEffect(() => {
    if (!!token) {
      getGallery();
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
          <Divider />
          <Heading
            as="h6"
            size="md"
            fontWeight={"450"***REMOVED***
            alignSelf={"flex-start"***REMOVED***
          >
            Manage My Gallery
            <Popover
              isOpen={isOpenAddGallery***REMOVED***
              onClose={() => {
                onCloseAddGallery();
 ***REMOVED*****REMOVED******REMOVED***
            >
              <PopoverTrigger>
                <Button
                  colorScheme="teal"
                  variant="ghost"
                  ml={4***REMOVED***
                  rightIcon={<GrAdd />***REMOVED***
                  onClick={() => {
                    onOpenAddGallery();
***REMOVED*****REMOVED*****REMOVED******REMOVED***
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
                    value={addGalleryName***REMOVED***
                    onChange={(event) => setAddGalleryName(event.target.value)***REMOVED***
                  />
                  <Button
                    colorScheme="twitter"
                    variant="outline"
                    width={"100%"***REMOVED***
                    mt={5***REMOVED***
                    onClick={() => {
                      addGallery();
  ***REMOVED*****REMOVED*****REMOVED******REMOVED***
                  >
                    Create
                  </Button>
                </PopoverBody>
              </PopoverContent>
            </Popover>
          </Heading>
 ***REMOVED*****REMOVED***galleryList?.length === 0 ? (
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
                  <Th>Color</Th>
                  <Th>Cover</Th>
                  <Th>Time Crd</Th>
                  <Th>Action</Th>
                </Tr>
              </Thead>
              <Tbody>
  ***REMOVED*****REMOVED*****REMOVED***galleryList?.map((gallery) => {
                  return (
                    <Tr key={gallery.gaId***REMOVED***>
                      <Td>{gallery.gaId***REMOVED***</Td>
                      <Td>{gallery.gaName***REMOVED***</Td>
                      <Td>
                        <Box
                          w="40px"
                          h="40px"
                          bg={gallery.coverColor***REMOVED***
                          borderRadius="25"
                        ></Box>
                      </Td>
                      <Td>{getCoverIcon(gallery.coverId)***REMOVED***</Td>
                      <Td>{gallery.createDate.substr(0, 10)***REMOVED***</Td>
                      <Td>
                        <Button
                          leftIcon={<GrUpdate />***REMOVED***
                          onClick={() => {
                            setEditGalleryName(gallery.gaName);
                            setEditGalleryId(gallery.gaId);
                            setEditGalleryColor(gallery.coverColor);
                            if (gallery.coverId === null) {
                              setEditGalleryCover(0);
***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED*** else {
                              setEditGalleryCover(gallery.coverId);
***REMOVED*****REMOVED*****REMOVED*****REMOVED*****REMOVED***

                            onOpenEditModal();
   ***REMOVED*****REMOVED*****REMOVED*****REMOVED******REMOVED***
                        ></Button>

                        <Button
                          leftIcon={<AiOutlineDelete />***REMOVED***
                          onClick={() => {
                            setDelGalleryId(gallery.gaId);
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
                Are you sure? You can't undo this action afterwards.
              </AlertDialogBody>

              <AlertDialogFooter>
                <Button ref={cancelRefBtn***REMOVED*** onClick={onCloseDeleteConfirm***REMOVED***>
                  Cancel
                </Button>
                <Button
                  colorScheme="red"
                  onClick={() => {
                    delGallery();
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
            <ModalHeader>Edit Gallery</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <Input
                variant="flushed"
                placeholder="Gallery New Name"
                value={editGalleryName***REMOVED***
                onChange={(event) => setEditGalleryName(event.target.value)***REMOVED***
              />
              <Input
                variant="filled"
                type={"color"***REMOVED***
                value={editGalleryColor***REMOVED***
                onChange={(event) => setEditGalleryColor(event.target.value)***REMOVED***
                mt={3***REMOVED***
              />
              <Select
                placeholder="Icon Format"
                mt={3***REMOVED***
                value={editGalleryCover***REMOVED***
                onChange={(e) => {
                  setEditGalleryCover(parseInt(e.target.value));
   ***REMOVED*****REMOVED******REMOVED***
              >
                <option value={0***REMOVED***>Default</option>
                <option value={1***REMOVED***>Star</option>
                <option value={2***REMOVED***>Favorite</option>
                <option value={3***REMOVED***>Beach</option>
                <option value={4***REMOVED***>Plane</option>
                <option value={5***REMOVED***>Flag</option>
                <option value={6***REMOVED***>Academic</option>
                <option value={7***REMOVED***>Mountain</option>
              </Select>
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
                  editGallery(editGalleryId);
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

export default GallerySetting;
