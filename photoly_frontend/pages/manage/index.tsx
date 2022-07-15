import { Stack ***REMOVED*** from "@chakra-ui/react";
import React, { useEffect, useState, useRef ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import Navbar from "../../components/Navbar";
import NavManager from "../../components/manage/NavManager";
import MyProfile from "../../components/manage/MyProfile";
import TagSetting from "../../components/manage/TagSetting";
import AdminSettings from "../../components/manage/AdminSettings";
import GallerySetting from "../../components/manage/GallerySetting";
import AdminUser from "../../components/manage/AdminUser";

const Manage: React.FC = () => {
  const token = useToken();
  const { get, post ***REMOVED*** = useApi();
  const [currentSelected, setCurrentSelected] = useState<number>(0);

  const changeSelection = (num: number) => {
    setCurrentSelected(num);
  ***REMOVED***;

  return (
    <>
      <Navbar />

      <Stack
        bg={"gray.50"***REMOVED***
        h={"calc(100% - 4rem)"***REMOVED***
        direction="row"
        w="100vw-4rem"
      >
        <NavManager changeSelection={changeSelection***REMOVED*** />
    ***REMOVED***currentSelected === 0 ? <MyProfile /> : ""***REMOVED***
    ***REMOVED***currentSelected === 1 ? <TagSetting /> : ""***REMOVED***
    ***REMOVED***currentSelected === 2 ? <GallerySetting /> : ""***REMOVED***
    ***REMOVED***currentSelected === 3 ? <AdminSettings /> : ""***REMOVED***
    ***REMOVED***currentSelected === 4 ? <AdminUser /> : ""***REMOVED***
      </Stack>
    </>
  );
***REMOVED***;
export default Manage;
