<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
import { ref, defineComponent, h, reactive } from 'vue';
import { useMessage, NIcon, NButton, NInput, NSelect } from "naive-ui";
import {
    Book20Regular as BookIcon,
    Person20Regular as PersonIcon,
    DrinkWine20Regular as WineIcon
} from "@vicons/fluent";

window.$message = useMessage();
const users = ref([]);
const openPasswordEditModal = ref(false);
const openDeleteModal = ref(false);
const operateUserId = ref(-1);
const operateUsername = ref("");
const deleteBtnLoading = ref(false);
const newPasswordVal = ref("");
const editBtnLoading = ref(false);

const useSearch = ref(false);
const searchVal = ref("")

const updateUsername = (userId, userName) => {
  axios({
      method: 'post',
      baseURL: '',
      url: import.meta.env.VITE_APP_BASE_URL + "/admin/resetUsername",
      headers: {
      "HRD-Token": localStorage.getItem("HRD-Token")
      },
      data: {
        "userId": userId,
        "userName": userName
      }
  }).then((response) => {
      if (response.data.msgCode === 200)
      {
        getUserPage(pagination.page, pagination.pageSize);
        window.$message.success("update successful!");
      }else {
          window.$message.error("Get User Information Error!")
      }
  })
  .catch(function (error) { // 请求失败处理
      window.$message.error(error);
  });
}

const updateEmail = (userId, email) => {
  axios({
      method: 'post',
      baseURL: '',
      url: import.meta.env.VITE_APP_BASE_URL + "/admin/resetUserEmail",
      headers: {
      "HRD-Token": localStorage.getItem("HRD-Token")
      },
      data: {
        "userId": userId,
        "email": email
      }
  }).then((response) => {
      if (response.data.msgCode === 200)
      {
        getUserPage(pagination.page, pagination.pageSize);
        window.$message.success("update successful!");
      }else {
          window.$message.error("Get User Information Error!")
      }
  })
  .catch(function (error) { // 请求失败处理
      window.$message.error(error);
  });
}

const updateRole = (userId, role) => {
  axios({
      method: 'post',
      baseURL: '',
      url: import.meta.env.VITE_APP_BASE_URL + "/admin/resetUserRole",
      headers: {
      "HRD-Token": localStorage.getItem("HRD-Token")
      },
      data: {
        "userId": userId,
        "role": role
      }
  }).then((response) => {
      if (response.data.msgCode === 200)
      {
        getUserPage(pagination.page, pagination.pageSize);
        window.$message.success("update successful!");
      }else {
          window.$message.error("Get User Information Error!")
      }
  })
  .catch(function (error) { // 请求失败处理
      window.$message.error(error);
  });
}

const updatePassword = (userId, userName) => {
  openPasswordEditModal.value = true;
  operateUserId.value = userId;
  operateUsername.value = userName;
}

const deleteUser = (userId, userName) => {
  openDeleteModal.value = true;
  operateUserId.value = userId;
  operateUsername.value = userName;
}

const doDelete = () => {
  deleteBtnLoading.value = true;
  axios({
      method: 'post',
      baseURL: '',
      url: import.meta.env.VITE_APP_BASE_URL + "/admin/deleteUser?userId=" + operateUserId.value,
      headers: {
      "HRD-Token": localStorage.getItem("HRD-Token")
      },
      data: {
      }
  }).then((response) => {
      if (response.data.msgCode === 200)
      {
        getUserPage(pagination.page, pagination.pageSize);
        window.$message.success("Delete successful!");
        openDeleteModal.value = false;
      }else {
          window.$message.error("Delete Error!")
      }
      deleteBtnLoading.value = false;
  })
  .catch(function (error) { // 请求失败处理
      window.$message.error(error);
      deleteBtnLoading.value = false;
  });
  
}

const doUpdatePassword = () => {
  editBtnLoading.value = true;
  axios({
      method: 'post',
      baseURL: '',
      url: import.meta.env.VITE_APP_BASE_URL + "/admin/resetUserPassword",
      headers: {
      "HRD-Token": localStorage.getItem("HRD-Token")
      },
      data: {
        "userId": operateUserId.value,
        "password": newPasswordVal.value
      }
  }).then((response) => {
      if (response.data.msgCode === 200)
      {
        getUserPage(pagination.page, pagination.pageSize);
        window.$message.success("Update successful!");
        openPasswordEditModal.value = false;
      }else {
          window.$message.error("Update Error!")
      }
      editBtnLoading.value = false;
  })
  .catch(function (error) { // 请求失败处理
      window.$message.error(error);
      editBtnLoading.value = false;
  });
}

const doSearch = () => {
  useSearch.value = true;
  searchUserPage(1, pagination.pageSize, searchVal.value);
}

const backToNormalIndexing = () => {
  useSearch.value = false;
  getUserPage(1, pagination.pageSize);
}

const createColumns = ({

}) => {
return [
    {
        title: 'User ID',
        key: 'userId'
    },
    {
        title: 'Username',
        key: 'userName',
        render(row, index) {
            return h(NInput, {
                value: row.userName,
                onUpdateValue(v) {
                    users.value[index].userName = v;
                },
                onBlur() {
                    updateUsername(users.value[index].userId, users.value[index].userName);
                }
            });
        }
    },
    {
        title: 'Email',
        key: 'email',
        render(row, index) {
            return h(NInput, {
                value: row.email,
                onUpdateValue(v) {
                    users.value[index].email = v;
                },
                onBlur () {
                  updateEmail(users.value[index].userId, users.value[index].email)
                }
            });
        }
    },
    {
        title: 'Create Date',
        key: 'createDate'
    },
    {
        title: 'Role',
        key: 'role',
        render(row, index) {
            return h(NSelect, {
                value: row.role,
                options: roleOptions,
                onUpdateValue(v) {
                  updateRole(users.value[index].userId, v)
                  users.value[index].role = v
                },
            });
        }
    },
    {
        title: 'UUID',
        key: 'uuid'
    },
    {
      title: "Password",
      key: "actions",
      render(row, index) {
          return h(
            NButton,
            {
                strong: true,
                tertiary: true,
                type: 'info',
                size: "small",
                onClick: () => updatePassword(users.value[index].userId, users.value[index].userName)
            },
            { default: () => "Change" },
            
          );
      }
    },
    {
      title: "Delete",
      key: "actions",
      render(row, index) {
          return h(
            NButton,
            {
                strong: true,
                tertiary: true,
                type: 'error',
                size: "small",
                onClick: () => deleteUser(users.value[index].userId, users.value[index].userName)
            },
            { default: () => "Delete User" },
            
          );
      }
    }
    ]
};

const pagination = reactive(
  {
    page: 1,
    pageSize: 10,
    pageCount: 5,
    showSizePicker: true,
    pageSizes: [10, 20, 30],
    onChange: (page) => {
      pagination.page = page;
      if (useSearch)
      {
        searchUserPage(page, pagination.pageSize, searchVal.value);
      }else {
        getUserPage(page, pagination.pageSize);
      }
      
    },
    onUpdatePageSize: (pageSize) => {
      pagination.pageSize = pageSize;
      pagination.page = 1;
      if (useSearch)
      {
        searchUserPage(pagination.page, pageSize, searchVal.value);
      }else {
        getUserPage(pagination.page, pageSize);
      }
      
    }
}
)
const columns = createColumns(
    {
        showUsername(row) {
            window.$message.info(`Username ${row.userName}`);
        }
    }
);

const roleOptions = [
  {
    label: "admin",
    value: "admin"
  },
  {
    label: "user",
    value: "user"
  }
]

const getUserPage = (page, rowsPerPage) => {
  axios({
      method: 'get',
      baseURL: '',
      url: import.meta.env.VITE_APP_BASE_URL + "/admin/getUserPage?rowsPerPage=" + rowsPerPage + "&page=" + page,
      headers: {
      "HRD-Token": localStorage.getItem("HRD-Token")
      },
      data: {
      }
  }).then((response) => {
      if (response.data.msgCode === 200)
      {
          users.value = response.data.t;
          pagination.pageCount = response.data.pageNum;
      }else {
          window.$message.error("Get User Information Error!")
      }
  })
  .catch(function (error) { // 请求失败处理
      window.$message.error(error);
  });
}

const searchUserPage = (page, rowsPerPage, query) => {
  axios({
          method: 'get',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/admin/searchUserPage?rowsPerPage=" + rowsPerPage + "&page=" + page + "&search=" + query,
          headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
          }
      }).then((response) => {
          if (response.data.msgCode === 200)
          {
              users.value = response.data.t;
              pagination.pageCount = response.data.pageNum;
          }else {
              window.$message.error("Get User Information Error!")
          }
      })
      .catch(function (error) { // 请求失败处理
          window.$message.error(error);
      });
}

getUserPage(1, pagination.pageSize);

</script>

<template>
  <div>
    <n-space vertical size="large" >
    <n-layout>
      <n-layout-header id="header1">
        <img src="@/assets/logo.ico" id="img1" @click="$router.push('/thepersonincharge/dashboard')"/>
        <n-gradient-text :size="20" :gradient="{
          from: 'rgb(255 255 255)',
          to: 'rgb(245 245 245)'
        }" id="photolyText" @click="$router.push('/thepersonincharge/dashboard')">PHOTOLY ADMIN</n-gradient-text>
        <div id="naviContainer">
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="$router.push('/thepersonincharge/dashboard')">
                Dashboard
            </n-button>
            <n-button class="naviBarBtn" quaternary round strong color="aliceblue" @click="$router.push('/thepersonincharge/usermanage')">
                User Mng
            </n-button>
            
        </div>
      </n-layout-header>
    </n-layout>
  </n-space>

  <div style="align-items: center; display: flex; align-content: center; 
    justify-content: center; margin-top: 5vh;">
    <n-space>
        <n-card title="" size="medium" style="border-radius: 10px; width: 85vw">
          <n-space vertical>
            <n-h2>User Management</n-h2>

            <n-input-group>
              <n-input :style="{ width: '20%' }" v-model:value="searchVal" @keyup.enter="doSearch"/>
              <n-button type="primary" ghost @click="doSearch" >
                Search
              </n-button>
              <n-button type="primary" ghost @click="backToNormalIndexing">
                Back to Indexing
              </n-button>
            </n-input-group>

            <n-data-table remote
                :columns="columns"
                :data="users"
                :pagination="pagination"
                :bordered="false"
                striped
                style="margin-top: 1rem;"
            />
          </n-space>
          
        </n-card>
    </n-space>
  </div>

  <!-- This part is for modal of edit password of an user -->
  <n-modal v-model:show="openPasswordEditModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Edit Password"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
      <n-space vertical>
        <n-p></n-p>
        <n-p>ID: {{ operateUserId }} | Username: {{ operateUsername }}</n-p>
        <n-space style="align-items: center;">
          <n-input placeholder="New Password" v-model:value="newPasswordVal" @keyup.enter="doUpdatePassword"></n-input>
        </n-space>
      </n-space>

        <n-space style="margin-top: 1rem">
          <n-button type="primary" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="doUpdatePassword" :loading="editBtnLoading">
            Submit
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {openPasswordEditModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
  </n-modal>
  <!-- ENDOF: This part is for modal of edit password of an user -->

  <!-- This part is for modal of confirming to delete an user -->
  <n-modal v-model:show="openDeleteModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Confirm to Delete"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        Do you really want to Delete this User with ID {{ operateUserId }} and Username {{ operateUsername }}?

        <n-space style="margin-top: 1rem">
          <n-button type="error" id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="doDelete" :loading="deleteBtnLoading">
            Delete
          </n-button>
          <n-button id="bt1" round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {openDeleteModal = false;}">
            Cancel
          </n-button>
        </n-space>

      </n-card>
  </n-modal>
  <!-- ENDOF: This part is for modal of confirming to delete an user -->
    
  </div>
</template>

<script>

export default defineComponent({
  data() {
    return {

    }
  },
  setup () {
  },
  methods: {
    checkIfIsAdmin () {
      axios({
        method: 'get',
        baseURL: '',
        url: import.meta.env.VITE_APP_BASE_URL + "/admin/isAdmin",
        headers: {
          "HRD-Token": localStorage.getItem("HRD-Token")
        },
        data: {
        }
      }).then((response) => {
        if (response.data.t)
        {
        }else {
          window.$message.error("You are not an admin!")
          $router.push('/login');
        }
      })
      .catch(function (error) { // 请求失败处理
        window.$message.error(error);
        $router.push('/login');
      });
    },
  },
  computed: {

  },
  mounted () {
    window.$message = useMessage();
    this.checkIfIsAdmin();
  },
  components: {

  },
  props: {
    
  }
})

</script>

<style scoped>
#header1 {
  font-size: 20px;
  background-color: #38b2ac;
  height: 4rem;
}
#img1 {
  width: 50px;
  border-radius: 30px;
  vertical-align: middle;
  padding-left: 0.4rem;
  padding-top: 0.4rem;
  padding-bottom: 0.4rem;
  margin-left: 10px;
  cursor: pointer;

  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}
#photolyText {
  vertical-align: middle;
  margin-left: 10px;
  cursor: pointer;

  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}
#naviContainer {
  float:right;
  padding-top: 0.5rem;
  margin-right: 20px;
  
}
.naviBarBtn {
  color: aliceblue;
  font-size: 1rem;
  margin-top: 0.4rem;
}
.naviBarBtn:hover {
  color: #EDF2F7;
}
#userImg {
  width: 45px;
  border-radius: 30px;
  vertical-align:top;


  cursor: pointer;

  -webkit-touch-callout: none;
  -webkit-user-select: none;
  -khtml-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
  
}
.n-menu .n-menu-item-content .n-menu-item-content-header {
  color: white;
}
</style>
