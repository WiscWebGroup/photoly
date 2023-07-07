import {Stack} from "@chakra-ui/react";
import React, {useState} from "react";
import useToken from "../../hooks/useToken";
import Navbar from "../../components/Navbar";
import NavManager from "../../components/manage/NavManager";
import MyProfile from "../../components/manage/MyProfile";
import TagSetting from "../../components/manage/TagSetting";
import AdminSettings from "../../components/manage/AdminSettings";
import GallerySetting from "../../components/manage/GallerySetting";
import AdminUser from "../../components/manage/AdminUser";

const Manage: React.FC = () => {
    useToken();
    const [currentSelected, setCurrentSelected] = useState<number>(0);

  const changeSelection = (num: number) => {
    setCurrentSelected(num);
  };

  return (
    <>
      <Navbar />

      <Stack
        bg={"gray.50"}
        h={"calc(100% - 4rem)"}
        direction="row"
        w="100vw-4rem"
      >
        <NavManager changeSelection={changeSelection} />
        {currentSelected === 0 ? <MyProfile /> : ""}
        {currentSelected === 1 ? <TagSetting /> : ""}
        {currentSelected === 2 ? <GallerySetting /> : ""}
        {currentSelected === 3 ? <AdminSettings /> : ""}
        {currentSelected === 4 ? <AdminUser /> : ""}
      </Stack>
    </>
  );
};
export default Manage;
